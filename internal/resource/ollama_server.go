package resource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// defaultOllamaHost is where the Ollama server listens when $OLLAMA_HOST is
	// unset — the same default the ollama CLI uses.
	defaultOllamaHost = "127.0.0.1:11434"

	// ollamaProbeTimeout bounds a single reachability probe. The server, when
	// up, answers /api/version almost instantly; a slow probe means it's down.
	ollamaProbeTimeout = 750 * time.Millisecond

	// ollamaReadyTimeout bounds how long we wait for a freshly started server to
	// begin answering. `ollama serve` normally binds within a second, but the
	// first-ever start can be slower while it initializes its store.
	ollamaReadyTimeout = 30 * time.Second

	// ollamaPollInterval is how often we re-probe while waiting for readiness.
	ollamaPollInterval = 200 * time.Millisecond
)

// ollamaProbeClient dials the server directly. A health check of a local server
// must not be routed through a configured HTTP proxy, which could otherwise
// answer (or refuse) on the real server's behalf and give a wrong verdict.
var ollamaProbeClient = &http.Client{Transport: &http.Transport{Proxy: nil}}

// ollamaServer lazily ensures a running Ollama server for the duration of an
// apply run. `ollama pull`/`ollama rm` are HTTP clients that need the server up;
// if none is reachable, this starts a temporary `ollama serve` and stops it when
// the run ends. It only ever stops a server it started itself — a server the
// user already has running (the desktop app or a manual `ollama serve`) is
// detected and left untouched.
//
// A single instance is shared by the pull and remove executors and is safe for
// concurrent use, though in practice all Ollama actions run in one serial chain
// (they share SerialGroup "ollama"), so ensure is only ever called sequentially.
type ollamaServer struct {
	mu     sync.Mutex
	ready  bool               // a usable server has been confirmed or started this run
	cmd    *exec.Cmd          // non-nil only when we started the server (and must stop it)
	cancel context.CancelFunc // cancels cmd's context, terminating the server

	// Injection points for tests; nil means use the real implementation.
	reachableFn func(ctx context.Context) bool
	startFn     func(ctx context.Context) (*exec.Cmd, context.CancelFunc, error)
}

// ensure guarantees a reachable Ollama server, starting a temporary one if
// needed. It is idempotent within a run: once a server is confirmed or started,
// subsequent calls return immediately. Progress notes are written to out (the
// action's output stream) so the user sees why a server is being started.
func (s *ollamaServer) ensure(ctx context.Context, out io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return nil
	}

	reachable := s.reachableFn
	if reachable == nil {
		reachable = ollamaReachableDefault
	}

	// A server the user already runs (desktop app or manual `ollama serve`) is
	// used as-is and never stopped by us.
	if reachable(ctx) {
		s.ready = true
		return nil
	}

	_, _ = fmt.Fprintln(out, "ollama server not running — starting a temporary one")

	start := s.startFn
	if start == nil {
		start = startOllamaServe
	}
	cmd, cancel, err := start(ctx)
	if err != nil {
		return fmt.Errorf("start ollama server: %w", err)
	}

	if err := waitOllamaReachable(ctx, reachable, ollamaReadyTimeout); err != nil {
		cancel()
		_ = cmd.Wait()
		return fmt.Errorf("ollama server did not become ready: %w", err)
	}

	s.cmd, s.cancel, s.ready = cmd, cancel, true
	return nil
}

// Close stops the temporary server if we started one, leaving a user-run server
// untouched. It resets state so the instance can be reused by a later run, and
// is safe to call multiple times.
func (s *ollamaServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ready = false
	if s.cmd == nil {
		return nil
	}
	s.cancel()
	_ = s.cmd.Wait() // reap; the signal-kill exit status is expected and ignored
	s.cmd, s.cancel = nil, nil
	return nil
}

// startOllamaServe starts `ollama serve` as a detached background process whose
// lifetime we control via the returned cancel func. The context is deliberately
// rooted at Background rather than the per-action context so the server outlives
// the single pull/remove that triggered its start — it must stay up across the
// whole Ollama chain and is torn down explicitly by ollamaServer.Close.
func startOllamaServe(_ context.Context) (*exec.Cmd, context.CancelFunc, error) {
	sctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(sctx, "ollama", "serve")
	// The server is chatty (it logs every request); its output isn't useful under
	// a model-pull's progress display, so discard it.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	// Prefer a graceful SIGTERM on cancel, falling back to a forced kill if the
	// server doesn't exit promptly.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, err
	}
	return cmd, cancel, nil
}

// waitOllamaReachable polls until the server answers or the timeout elapses.
func waitOllamaReachable(ctx context.Context, reachable func(context.Context) bool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(ollamaPollInterval)
	defer ticker.Stop()

	for {
		if reachable(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ollamaReachableDefault reports whether an Ollama server answers at the address
// the CLI will connect to (honoring $OLLAMA_HOST).
func ollamaReachableDefault(ctx context.Context) bool {
	return ollamaReachable(ctx, ollamaBaseURL())
}

// ollamaReachable reports whether an HTTP GET to <baseURL>/api/version completes.
// Any completed round-trip means a server is listening; a connection error means
// it is not.
func ollamaReachable(ctx context.Context, baseURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, ollamaProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := ollamaProbeClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// ollamaBaseURL resolves the server base URL from $OLLAMA_HOST, matching how the
// ollama CLI interprets it: a bare "host:port" (or ":port", or "host") gets an
// http:// scheme; a value that already carries a scheme is used verbatim.
func ollamaBaseURL() string {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		host = defaultOllamaHost
	}
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}
	return "http://" + host
}
