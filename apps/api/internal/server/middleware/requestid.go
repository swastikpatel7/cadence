package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"net/http"
	"strings"
)

type ctxKey struct{}

const HeaderRequestID = "X-Request-Id"

// RequestID assigns a request ID to every request. If the client sent
// one in X-Request-Id, we honour it; otherwise we generate a fresh one.
// The ID is echoed back in the response header for client correlation.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(HeaderRequestID)
		if rid == "" {
			rid = generate()
		}
		w.Header().Set(HeaderRequestID, rid)
		ctx := context.WithValue(r.Context(), ctxKey{}, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the request ID stored on ctx, or "" if none.
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

func generate() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to empty string; downstream code is resilient to it.
		return ""
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}
