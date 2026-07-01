package resource

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestOllamaBaseURL(t *testing.T) {
	cases := []struct {
		name string
		host string
		want string
	}{
		{"default when unset", "", "http://127.0.0.1:11434"},
		{"host and port", "192.168.1.10:11434", "http://192.168.1.10:11434"},
		{"port only", ":9999", "http://:9999"},
		{"bare host", "myhost", "http://myhost"},
		{"explicit scheme", "https://ollama.internal:443", "https://ollama.internal:443"},
		{"scheme with trailing slash", "http://127.0.0.1:11434/", "http://127.0.0.1:11434"},
		{"whitespace trimmed", "  host:1234  ", "http://host:1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OLLAMA_HOST", tc.host)
			if got := ollamaBaseURL(); got != tc.want {
				t.Errorf("ollamaBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOllamaReachable(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if !ollamaReachable(context.Background(), ts.URL) {
		t.Errorf("expected running server to be reachable")
	}

	// An address with nothing listening yields a connection error → not reachable.
	if ollamaReachable(context.Background(), "http://127.0.0.1:1") {
		t.Errorf("expected closed port to be unreachable")
	}
}

// alwaysReachable is a reachability stub that always reports up.
func alwaysReachable(context.Context) bool { return true }

func TestEnsureUsesExistingServer(t *testing.T) {
	var started atomic.Bool
	s := &ollamaServer{
		reachableFn: alwaysReachable,
		startFn: func(context.Context) (*exec.Cmd, context.CancelFunc, error) {
			started.Store(true)
			return nil, nil, errors.New("should not start")
		},
	}

	var out bytes.Buffer
	if err := s.ensure(context.Background(), &out); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if started.Load() {
		t.Errorf("start should not be called when a server is already reachable")
	}
	if out.Len() != 0 {
		t.Errorf("no note should be emitted when a server is already running, got %q", out.String())
	}
	// We don't own the server, so Close must not try to stop anything.
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestEnsureStartsAndStopsTemporaryServer(t *testing.T) {
	// Reachable only after the first probe, simulating a server that comes up
	// once started.
	var probes atomic.Int32
	reachable := func(context.Context) bool {
		return probes.Add(1) > 1
	}

	var startCount atomic.Int32
	var cmd *exec.Cmd
	s := &ollamaServer{
		reachableFn: reachable,
		startFn: func(context.Context) (*exec.Cmd, context.CancelFunc, error) {
			startCount.Add(1)
			c, cancel, err := startStubServer()
			cmd = c
			return c, cancel, err
		},
	}

	var out bytes.Buffer
	if err := s.ensure(context.Background(), &out); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !strings.Contains(out.String(), "starting a temporary one") {
		t.Errorf("expected a start note, got %q", out.String())
	}
	if cmd == nil || cmd.Process == nil {
		t.Fatalf("expected a started process")
	}

	// Idempotent: a second ensure neither re-probes nor re-starts.
	if err := s.ensure(context.Background(), &out); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if got := startCount.Load(); got != 1 {
		t.Errorf("start called %d times, want 1", got)
	}

	// Close stops and reaps the process we started.
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if cmd.ProcessState == nil {
		t.Errorf("expected the temporary server to be stopped and reaped")
	}

	// Close is idempotent.
	if err := s.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestEnsureStartFailureSurfaces(t *testing.T) {
	s := &ollamaServer{
		reachableFn: func(context.Context) bool { return false },
		startFn: func(context.Context) (*exec.Cmd, context.CancelFunc, error) {
			return nil, nil, errors.New("boom")
		},
	}
	err := s.ensure(context.Background(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "start ollama server") {
		t.Fatalf("expected start error, got %v", err)
	}
}

func TestWaitOllamaReachableTimeout(t *testing.T) {
	t.Parallel()
	never := func(context.Context) bool { return false }
	err := waitOllamaReachable(context.Background(), never, 100*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRegistryCloseNoOp(t *testing.T) {
	// A fresh default registry never started a server, so Close is a clean no-op.
	if err := DefaultRegistry().Close(); err != nil {
		t.Errorf("Registry.Close on unused registry: %v", err)
	}
}

// startStubServer starts a long-lived process standing in for `ollama serve`,
// terminated the same way the real one is (SIGTERM via the cancel context).
func startStubServer() (*exec.Cmd, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sleep", "30")
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, err
	}
	return cmd, cancel, nil
}
