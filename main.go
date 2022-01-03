package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/outblocks/outblocks-cli/cmd"
	"google.golang.org/grpc/grpclog"
)

func main() {
	// Disable grpc client logging.
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))

	exec := cmd.NewExecutor()

	ctx, cancel := context.WithCancel(context.Background())

	// Handle SIGINT and SIGTERM.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ch

		exec.Log().Warnln("Signal received. Canceling execution...")
		cancel()
	}()

	defer func() {
		if r := recover(); r != nil {
			exec.Log().Errorf("Critical Error! %q\n%s", r, debug.Stack())

			os.Exit(1)
		}
	}()

	err := exec.Execute(ctx)

	cancel()

	if err != nil {
		exec.Log().Errorln(err)

		os.Exit(1) // nolint: gocritic
	}
}
