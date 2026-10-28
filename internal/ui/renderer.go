package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ryanwersal/crucible/internal/action"
)

type actionStatus int

const (
	statusPending actionStatus = iota
	statusRunning
	statusDone
	statusFailed
)

type actionState struct {
	action      action.Action
	status      actionStatus
	lines       []string // last N output lines
	err         error
	spinnerTick int
}

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// Renderer is an ActionObserver that draws a live-updating terminal display.
// Live frames show active actions; the final report includes captured failures.
//
// Lifecycle: NewRenderer → Start → (ActionStarted/Output/Completed) → Wait.
// Start and Wait must be called from the same goroutine; the observer methods
// are safe for concurrent use from multiple goroutines.
type Renderer struct {
	mu            sync.Mutex
	w             io.Writer
	actions       []actionState
	maxLines      int // max output lines per action
	termWidth     int
	lastLineCount int
	done          chan struct{}
	loopDone      sync.WaitGroup // signaled when the render loop exits
	stopOnce      sync.Once
	cancel        context.CancelFunc
}

// NewRenderer creates a renderer that writes to the given terminal.
// actions initializes the pending rows before execution begins.
func NewRenderer(w *os.File, actions []action.Action, maxLines int) *Renderer {
	states := make([]actionState, len(actions))
	for i, a := range actions {
		states[i].action = a
	}
	return &Renderer{
		w:         w,
		actions:   states,
		maxLines:  maxLines,
		termWidth: terminalWidth(w),
		done:      make(chan struct{}),
	}
}

// Start begins the render loop. The loop stops when Wait is called or the
// context is cancelled.
func (r *Renderer) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)

	// Hide cursor.
	_, _ = fmt.Fprint(r.w, "\033[?25l")

	r.loopDone.Add(1)
	go r.loop(ctx)
}

func (r *Renderer) loop(ctx context.Context) {
	defer r.loopDone.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.done:
			r.render()
			return
		case <-ctx.Done():
			r.render()
			return
		case <-ticker.C:
			r.renderProgress()
		}
	}
}

// ActionStarted implements ActionObserver.
func (r *Renderer) ActionStarted(index int, a action.Action) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[index] = actionState{
		action: a,
		status: statusRunning,
	}
}

// ActionOutput implements ActionObserver.
func (r *Renderer) ActionOutput(index int, line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &r.actions[index]
	s.lines = append(s.lines, line)
	if len(s.lines) > r.maxLines {
		s.lines = s.lines[len(s.lines)-r.maxLines:]
	}
}

// ActionCompleted implements ActionObserver.
func (r *Renderer) ActionCompleted(index int, a action.Action, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &r.actions[index]
	if err != nil {
		s.status = statusFailed
		s.err = err
	} else {
		s.status = statusDone
	}
}

// Wait signals the render loop to perform a final render, blocks until it
// completes, and restores the cursor. Safe to call multiple times.
func (r *Renderer) Wait() {
	r.stopOnce.Do(func() {
		close(r.done)
		r.loopDone.Wait()
		if r.cancel != nil {
			r.cancel()
		}
		// Restore cursor.
		_, _ = fmt.Fprint(r.w, "\033[?25h")
	})
}

func (r *Renderer) render() {
	r.mu.Lock()
	defer r.mu.Unlock()

	var buf strings.Builder

	// Move cursor up to overwrite previous frame.
	if r.lastLineCount > 0 {
		fmt.Fprintf(&buf, "\033[%dA\r\033[J", r.lastLineCount)
	}

	var completed, failed int

	for i := range r.actions {
		s := &r.actions[i]

		switch s.status {
		case statusPending:
			buf.WriteString(cleanTerminalText(fmt.Sprintf("  ○ %s", s.action.Description)))
			buf.WriteString("\033[K\n")

		case statusRunning:
			frame := spinnerFrames[s.spinnerTick%len(spinnerFrames)]
			s.spinnerTick++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			buf.WriteString(cleanTerminalText(fmt.Sprintf("  %s %s", frame, desc)))
			buf.WriteString("\033[K\n")
			for _, line := range s.lines {
				buf.WriteString(cleanTerminalText(fmt.Sprintf("    %s", line)))
				buf.WriteString("\033[K\n")
			}

		case statusDone:
			completed++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			buf.WriteString(cleanTerminalText(fmt.Sprintf("  ✓ %s", desc)))
			buf.WriteString("\033[K\n")

		case statusFailed:
			completed++
			failed++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			// Use the first line of the error so multi-line CommandError
			// messages don't blow up the headline; the captured tail is
			// shown via the trailing output lines below.
			headline := firstLine(s.err.Error())
			buf.WriteString(cleanTerminalText(fmt.Sprintf("  ✗ %s: %s", desc, headline)))
			buf.WriteString("\033[K\n")
			// Keep the captured subprocess output visible below the failure
			// marker so the user can diagnose without re-running.
			for _, line := range failureLines(s) {
				buf.WriteString(cleanTerminalText(fmt.Sprintf("    │ %s", line)))
				buf.WriteString("\033[K\n")
			}
		}
	}

	// Summary footer.
	total := len(r.actions)
	summary := fmt.Sprintf("  [%d/%d complete", completed, total)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	summary += "]"
	buf.WriteString(cleanTerminalText(summary))
	buf.WriteString("\033[K\n")

	r.lastLineCount = 0 // Final output is static and may wrap or scroll.
	_, _ = fmt.Fprint(r.w, buf.String())
}

// firstLine returns the portion of s up to the first newline (without the
// newline itself). Multi-line error messages use the rest of the lines for
// detail; the headline gets just the summary.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (r *Renderer) truncate(s string) string {
	// Count visible runes (skip ANSI escape sequences).
	visible := 0
	inEsc := false
	for i, c := range s {
		if c == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		visible++
		if visible >= r.termWidth {
			return s[:i+utf8.RuneLen(c)]
		}
	}
	return s
}

// renderProgress redraws only active action names. Subprocess output is retained
// for failures, rather than repeatedly filling the terminal with progress logs.
func (r *Renderer) renderProgress() {
	r.mu.Lock()
	defer r.mu.Unlock()
	height := 24
	if f, ok := r.w.(*os.File); ok {
		r.termWidth = terminalWidth(f)
		height = terminalHeight(f)
	}
	var buf strings.Builder
	if r.lastLineCount > 0 {
		fmt.Fprintf(&buf, "\033[%dA\r\033[J", r.lastLineCount)
	}
	completed, failed, running, shown := 0, 0, 0, 0
	for i := range r.actions {
		s := &r.actions[i]
		switch s.status {
		case statusDone:
			completed++
		case statusFailed:
			completed++
			failed++
		case statusRunning:
			running++
			if shown >= max(0, height-3) {
				continue
			}
			frame := spinnerFrames[s.spinnerTick%len(spinnerFrames)]
			s.spinnerTick++
			fmt.Fprintf(&buf, "%s\033[0m\033[K\n", r.truncate("  "+frame+" "+strings.ReplaceAll(cleanTerminalText(s.action.Description), "\n", " ")))
			shown++
		}
	}
	summary := fmt.Sprintf("  [%d/%d complete, %d failed, %d running, %d pending]", completed, len(r.actions), failed, running, len(r.actions)-completed-running)
	fmt.Fprintf(&buf, "%s\033[0m\033[K\n", r.truncate(summary))
	r.lastLineCount = shown + 1
	_, _ = fmt.Fprint(r.w, buf.String())
}

var terminalEscapes = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]|\x1b\\][^\a\x1b]*(?:\a|\x1b\\\\)|\x1b[@-Z\\\\-_]")

func cleanTerminalText(s string) string {
	s = terminalEscapes.ReplaceAllString(s, "")
	return strings.Map(func(c rune) rune {
		if c == '\t' {
			return ' '
		}
		if c < 32 && c != '\n' {
			return -1
		}
		if c == 127 {
			return -1
		}
		return c
	}, s)
}

func failureLines(s *actionState) []string {
	if carrier, ok := errors.AsType[outputCarrier](s.err); ok && carrier.OutputTail() != "" {
		return strings.Split(strings.TrimRight(cleanTerminalText(carrier.OutputTail()), "\n"), "\n")
	}
	if len(s.lines) == 0 {
		if _, body, ok := strings.Cut(s.err.Error(), "\n"); ok {
			return strings.Split(cleanTerminalText(body), "\n")
		}
	}
	return s.lines
}
