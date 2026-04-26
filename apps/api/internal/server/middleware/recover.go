package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches panics in downstream handlers, logs them with stack,
// and returns a generic 500. This is the panic firewall — it must wrap
// every other middleware.
func Recover(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					base.Error("panic recovered",
						"panic", rec,
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", FromContext(r.Context()),
						"stack", string(debug.Stack()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL","message":"internal server error"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
