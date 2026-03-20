package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryanwersal/crucible/internal/cli"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := cli.NewRootCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs(cli.RewriteScriptArgs(os.Args[1:], cli.SubcommandNames(cmd)))

	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}
