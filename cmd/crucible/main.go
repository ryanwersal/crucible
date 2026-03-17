package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryanwersal/crucible/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := cli.NewRootCmd()
	cmd.SetContext(ctx)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
