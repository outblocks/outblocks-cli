package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/outblocks/outblocks-cli/cmd"
	"github.com/outblocks/outblocks-cli/pkg/actions"
	"google.golang.org/grpc/grpclog"
)

func main() {
	// Disable grpc client logging.
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))

	exec := cmd.NewExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	if err != nil {
		if e, ok := err.(*actions.ErrExit); ok {
			if e.Message != "" {
				exec.Log().Errorln(e.Message)
			}

			os.Exit(e.StatusCode) //nolint
		}

		if ctx.Err() != context.Canceled {
			exec.Log().Errorln("Error occurred:", err)
		}

		os.Exit(1)
	}
}
