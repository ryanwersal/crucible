package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// humanHandler is an slog.Handler that produces concise, human-readable output.
//
// INFO messages print only the message text and any attributes on the same line.
// Other levels are prefixed with a short tag (DBG, WRN, ERR).
type humanHandler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Leveler
	attrs []slog.Attr
}

func newHumanHandler(w io.Writer, level slog.Leveler) *humanHandler {
	return &humanHandler{w: w, mu: &sync.Mutex{}, level: level}
}

func (h *humanHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *humanHandler) Handle(_ context.Context, r slog.Record) error {
	var buf []byte

	switch {
	case r.Level >= slog.LevelError:
		buf = append(buf, "ERR "...)
	case r.Level >= slog.LevelWarn:
		buf = append(buf, "WRN "...)
	case r.Level >= slog.LevelDebug && r.Level < slog.LevelInfo:
		buf = append(buf, "DBG "...)
	}

	buf = append(buf, r.Message...)

	appendAttr := func(a slog.Attr) {
		if a.Equal(slog.Attr{}) {
			return
		}
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
	}

	for _, a := range h.attrs {
		appendAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(a)
		return true
	})

	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return fmt.Errorf("write log: %w", err)
}

func (h *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &humanHandler{
		w:     h.w,
		mu:    h.mu,
		level: h.level,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *humanHandler) WithGroup(_ string) slog.Handler {
	// Groups are not used in this codebase; return as-is.
	return h
}
