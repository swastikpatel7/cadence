// Cadence API entrypoint.
//
// main is intentionally tiny: it loads dependencies via the composition
// root (internal/system), starts the HTTP server and the River worker
// pool inside the same process, and shuts both down on SIGINT / SIGTERM
// with a 30-second drain window.
//
// Why HTTP + River share a process: at v1 traffic the API and the
// background queue have plenty of headroom on a single instance, and
// running them together drops one Railway service, one Dockerfile, one
// railway.toml, and an entire env-var set. If the worker ever needs
// independent scaling, split it back out.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/swastikpatel7/cadence/apps/api/internal/system"
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

	// Start River first so it's ready by the time the HTTP server
	// accepts traffic that may enqueue jobs.
	if err := deps.River.Start(ctx); err != nil {
		deps.Logger.Error("river start failed", "err", err)
		os.Exit(1)
	}
	deps.Logger.Info("river started, processing jobs")

	deps.Logger.Info("api listening", "addr", deps.Server.Addr)
	serverErr := make(chan error, 1)
	go func() {
		if err := deps.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		deps.Logger.Info("shutdown signal received, draining")
	case err := <-serverErr:
		if err != nil {
			deps.Logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}

	// Order matters: stop accepting HTTP first so no new jobs are
	// enqueued, then drain in-flight jobs, then let deferred Close()
	// release the DB pool.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()

	if err := deps.Server.Shutdown(shutdownCtx); err != nil {
		deps.Logger.Error("graceful shutdown failed", "err", err)
	}

	if err := deps.River.Stop(shutdownCtx); err != nil {
		deps.Logger.Error("river stop failed", "err", err)
	}

	deps.Logger.Info("api stopped cleanly")
}
