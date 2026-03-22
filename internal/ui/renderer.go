package ui

import (
	"context"
	"fmt"
	"io"
	"os"
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
// Each action shows its description, status indicator, and trailing output lines.
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
// total is the number of actions that will be observed.
func NewRenderer(w *os.File, total int, maxLines int) *Renderer {
	return &Renderer{
		w:         w,
		actions:   make([]actionState, total),
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
			r.render()
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
		fmt.Fprintf(&buf, "\033[%dA", r.lastLineCount)
	}

	lineCount := 0
	var completed, failed int

	for i := range r.actions {
		s := &r.actions[i]

		switch s.status {
		case statusPending:
			buf.WriteString(r.truncate(fmt.Sprintf("  \033[2m○ %s\033[0m", s.action.Description)))
			buf.WriteString("\033[K\n")
			lineCount++

		case statusRunning:
			frame := spinnerFrames[s.spinnerTick%len(spinnerFrames)]
			s.spinnerTick++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			buf.WriteString(r.truncate(fmt.Sprintf("  \033[36m%s %s\033[0m", frame, desc)))
			buf.WriteString("\033[K\n")
			lineCount++
			for _, line := range s.lines {
				buf.WriteString(r.truncate(fmt.Sprintf("    \033[2m%s\033[0m", line)))
				buf.WriteString("\033[K\n")
				lineCount++
			}

		case statusDone:
			completed++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			buf.WriteString(r.truncate(fmt.Sprintf("  \033[32m✓ %s\033[0m", desc)))
			buf.WriteString("\033[K\n")
			lineCount++

		case statusFailed:
			completed++
			failed++
			desc := s.action.Description
			if s.action.NeedsSudo {
				desc = "[sudo] " + desc
			}
			buf.WriteString(r.truncate(fmt.Sprintf("  \033[31m✗ %s: %v\033[0m", desc, s.err)))
			buf.WriteString("\033[K\n")
			lineCount++
		}
	}

	// Summary footer.
	total := len(r.actions)
	summary := fmt.Sprintf("  [%d/%d complete", completed, total)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}
	summary += "]"
	buf.WriteString(r.truncate(summary))
	buf.WriteString("\033[K\n")
	lineCount++

	// Clear any leftover lines from previous frame.
	for i := lineCount; i < r.lastLineCount; i++ {
		buf.WriteString("\033[K\n")
		lineCount++
	}

	r.lastLineCount = lineCount
	_, _ = fmt.Fprint(r.w, buf.String())
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
