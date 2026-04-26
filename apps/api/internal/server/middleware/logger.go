package middleware

import (
	"log/slog"
	"net/http"
	"time"

	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// Logger logs each request after it completes and attaches a
// request-scoped *slog.Logger to the request context, so handlers and
// repos can fetch it via pkg/logger.FromContext.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			l := base
			if rid := FromContext(r.Context()); rid != "" {
				l = l.With("request_id", rid)
			}
			ctx := pkglogger.WithContext(r.Context(), l)

			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r.WithContext(ctx))

			l.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
