package resource

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ryanwersal/crucible/internal/action"
)

// maxCapturedOutputBytes bounds how much subprocess output we retain for
// inclusion in error messages. Roughly enough to capture a typical brew error
// stanza without unbounded memory growth on chatty commands.
const maxCapturedOutputBytes = 16 * 1024

// CommandError describes a failed subprocess invocation, carrying the captured
// tail of its combined stdout/stderr so callers can produce actionable error
// messages instead of just "exit status 1".
type CommandError struct {
	Command string   // executable name, e.g. "brew"
	Args    []string // arguments passed to the executable (sudo prefix not included)
	Err     error    // the underlying error from exec.Cmd.Run
	Output  string   // tail of combined stdout+stderr captured during execution
}

// Error renders a human-readable message including the captured output tail.
// The command name is intentionally omitted from the rendered string — callers
// surface a more contextual description (e.g. "brew install foo") around it,
// so prefixing the command again would just be noise.
func (e *CommandError) Error() string {
	out := strings.TrimRight(e.Output, "\n")
	if out == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v\n%s", e.Err, indentLines(out, "    "))
}

// CommandLine returns the full invocation as a single string ("name a b c").
// Useful when displaying CommandError in contexts that don't already name the
// failing command.
func (e *CommandError) CommandLine() string {
	if len(e.Args) == 0 {
		return e.Command
	}
	return e.Command + " " + strings.Join(e.Args, " ")
}

// OutputTail returns the captured stdout+stderr tail. Implements the
// outputCarrier interface used by observers to surface output separately.
func (e *CommandError) OutputTail() string { return e.Output }

// Unwrap exposes the underlying exec error so errors.Is / errors.As keep working.
func (e *CommandError) Unwrap() error { return e.Err }

// runCmd executes a subprocess for an action, teeing its stdout and stderr to
// the caller-provided writers AND a bounded ring buffer. If the process exits
// non-zero (or fails to start), the returned error is a *CommandError carrying
// the captured output tail. Context cancellation is surfaced as the raw exec
// error so callers can detect it via errors.Is(err, context.Canceled).
func runCmd(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	capture := newRingBuffer(maxCapturedOutputBytes)

	// exec.Cmd spawns a separate copy goroutine for each non-*os.File writer it
	// is given, so when the caller hands us the same writer for both streams
	// (the common case for observers and buffer-based tests) those goroutines
	// would race on the shared writer. Serialise them through a single
	// mutex-guarded fan-out when we detect that aliasing.
	stdoutW, stderrW := teeOptional(stdout, capture), teeOptional(stderr, capture)
	if stdout != nil && stdout == stderr {
		shared := &lockedWriter{w: stdoutW}
		stdoutW, stderrW = shared, shared
	}

	cmd := buildCmd(ctx, a, stdin, stdoutW, stderrW, name, args...)

	err := cmd.Run()
	if err == nil {
		return nil
	}

	// Don't wrap context cancellation — the output tail is rarely meaningful
	// when the user killed the process mid-flight.
	if ctx.Err() != nil {
		return err
	}

	return &CommandError{
		Command: name,
		Args:    args,
		Err:     err,
		Output:  capture.String(),
	}
}

// lockedWriter serialises Write calls to an underlying writer. Used when the
// same caller-provided writer is shared between a process's stdout and stderr
// pipes; exec.Cmd would otherwise invoke Write concurrently from two copy
// goroutines and race on writer state that's not goroutine-safe.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// teeOptional returns a writer that fans out to both w and capture. Either may
// be nil; if both are nil the result discards.
func teeOptional(w io.Writer, capture io.Writer) io.Writer {
	switch {
	case w == nil && capture == nil:
		return io.Discard
	case w == nil:
		return capture
	case capture == nil:
		return w
	default:
		return io.MultiWriter(w, capture)
	}
}

// ringBuffer keeps the most recent N bytes written to it. Concurrent writes
// from a process's stdout and stderr pipes are serialised under a mutex so
// the captured tail is well-defined.
type ringBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newRingBuffer(max int) *ringBuffer {
	initialCap := max
	if initialCap > 4096 {
		initialCap = 4096
	}
	return &ringBuffer{max: max, buf: make([]byte, 0, initialCap)}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.max {
		// Reslice onto a fresh array so the underlying storage can shrink and
		// dropped bytes can be garbage collected.
		excess := len(r.buf) - r.max
		newBuf := make([]byte, r.max)
		copy(newBuf, r.buf[excess:])
		r.buf = newBuf
	}
	return len(p), nil
}

func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

// indentLines prefixes every line of s with prefix. The trailing newline (if
// present) is preserved as-is — we only add prefixes between line breaks.
func indentLines(s, prefix string) string {
	if s == "" {
		return ""
	}
	var out bytes.Buffer
	out.Grow(len(s) + len(prefix)*8)
	out.WriteString(prefix)
	for i := range len(s) {
		out.WriteByte(s[i])
		if s[i] == '\n' && i != len(s)-1 {
			out.WriteString(prefix)
		}
	}
	return out.String()
}
