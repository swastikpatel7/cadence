// Package logger provides a thin wrapper over log/slog with conventions
// for Cadence: JSON in production, human-readable in development, and a
// context-keyed logger so request-scoped fields propagate via context.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

// Env names the runtime environment. "development" produces text output,
// anything else produces JSON.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
)

// New constructs a *slog.Logger configured for env at the given level.
// level accepts: "debug", "info", "warn", "error" (case-insensitive).
// Unknown levels default to info.
func New(env Env, level string) *slog.Logger {
	return NewWithWriter(env, level, os.Stdout)
}

// NewWithWriter is like New but writes to w. Useful in tests.
func NewWithWriter(env Env, level string, w io.Writer) *slog.Logger {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	if env == EnvDevelopment {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromContext returns the logger stored in ctx, or slog.Default() if none.
// Handlers and middleware should use this rather than passing a logger
// through every function signature.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithContext returns a new context that carries logger.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// With returns a logger from ctx with the given attributes attached.
// Equivalent to FromContext(ctx).With(attrs...).
func With(ctx context.Context, attrs ...any) *slog.Logger {
	return FromContext(ctx).With(attrs...)
}
