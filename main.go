package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/outblocks/outblocks-cli/cmd"
)

func main() {
	exec := cmd.NewExecutor()

	ctx, cancel := context.WithCancel(context.Background())

	// Handle SIGINT and SIGTERM.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ch

		exec.Log().Warnln("Canceling execution... Please wait for all pending tasks to finish...")
		cancel()
	}()

	defer func() {
		if r := recover(); r != nil {
			exec.Log().Errorf("Critical Error! %q\n%s", r, debug.Stack())
		}
	}()

	err := exec.Execute(ctx)

	cancel()

	if err != nil {
		exec.Log().Errorln(err)

		os.Exit(1) // nolint: gocritic
	}
}
