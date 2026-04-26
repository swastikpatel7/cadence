// Cadence worker entrypoint.
//
// Loads dependencies via the composition root, starts River, processes
// jobs from the default queue, and shuts down cleanly on SIGINT/SIGTERM
// with a 30-second drain.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/swastikpatel7/cadence/apps/worker/internal/system"
)

const drainTimeout = 30 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := system.InitDependencies(ctx)
	if err != nil {
		slog.Error("init failed", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			deps.Logger.Error("close failed", "err", err)
		}
	}()

	deps.Logger.Info("worker starting")

	if err := deps.River.Start(ctx); err != nil {
		deps.Logger.Error("river start failed", "err", err)
		os.Exit(1)
	}

	deps.Logger.Info("worker ready, processing jobs")
	<-ctx.Done()
	deps.Logger.Info("shutdown signal received, draining")

	stopCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	if err := deps.River.Stop(stopCtx); err != nil {
		deps.Logger.Error("river stop failed", "err", err)
		os.Exit(1)
	}

	deps.Logger.Info("worker stopped cleanly")
}
