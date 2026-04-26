// Cadence API entrypoint.
//
// main is intentionally tiny: it loads dependencies via the composition
// root (internal/system), starts the HTTP server, and shuts it down on
// SIGINT / SIGTERM with a 30-second drain window.
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

const shutdownTimeout = 30 * time.Second

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := deps.Server.Shutdown(shutdownCtx); err != nil {
		deps.Logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	deps.Logger.Info("api stopped cleanly")
}
