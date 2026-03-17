package script

import "log/slog"

// slogPrinter adapts slog.Logger to the goja_nodejs console.Printer interface.
type slogPrinter struct {
	logger *slog.Logger
}

func (p *slogPrinter) Log(msg string) {
	p.logger.Info(msg, "source", "console")
}

func (p *slogPrinter) Warn(msg string) {
	p.logger.Warn(msg, "source", "console")
}

func (p *slogPrinter) Error(msg string) {
	p.logger.Error(msg, "source", "console")
}
