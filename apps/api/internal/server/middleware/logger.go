package middleware

import (
	"bufio"
	"log/slog"
	"net"
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

// Unwrap exposes the underlying writer so http.NewResponseController can
// walk the chain. Required so SSE handlers (and anyone else) can fetch
// Flush/Hijack via the controller even when a wrapper sits in front.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush delegates to the underlying writer when it supports http.Flusher.
// Without this method, the SSE handler's `w.(http.Flusher)` assertion
// fails (embedding does not promote a method that isn't on the embedded
// interface — http.ResponseWriter doesn't include Flush), and the entire
// onboarding stream returns 500 before emitting a single event.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets WebSocket / connection-takeover handlers reach the
// underlying http.Hijacker through this wrapper. Same reasoning as
// Flush — embedding alone doesn't promote the optional interface.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
