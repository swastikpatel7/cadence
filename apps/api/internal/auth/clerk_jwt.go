// Package auth implements Clerk session JWT verification.
//
// Flow per request:
//   1. Read "Authorization: Bearer <jwt>"
//   2. Verify signature against Clerk's JWKS (cached, auto-refreshed)
//   3. Verify issuer + expiry
//   4. Extract Clerk's `sub` claim (the clerk_user_id)
//   5. Resolve to our internal user UUID — Redis cache-aside, DB on miss
//   6. Inject the internal UUID into ctx
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/redis/go-redis/v9"

	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

type userIDCtxKey struct{}

const (
	bearerPrefix    = "Bearer "
	headerAuth      = "Authorization"
	cacheKeyPrefix  = "auth:user_id:"
	cacheTTL        = 5 * time.Minute
	refreshInterval = 10 * time.Minute
)

// Verifier resolves Clerk session JWTs to internal user UUIDs.
// Safe for concurrent use.
type Verifier struct {
	issuer    string
	jwksURL   string
	jwksCache *jwk.Cache
	queries   *dbgen.Queries
	redis     *redis.Client
}

// NewVerifier constructs a Verifier and warms the JWKS cache. Returns
// an error if Clerk's JWKS endpoint is unreachable at startup.
func NewVerifier(
	ctx context.Context,
	jwksURL, issuer string,
	queries *dbgen.Queries,
	rdb *redis.Client,
) (*Verifier, error) {
	if jwksURL == "" {
		return nil, errors.New("auth: jwks URL required")
	}
	if issuer == "" {
		return nil, errors.New("auth: issuer required")
	}
	cache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("auth: init jwks cache: %w", err)
	}
	if err := cache.Register(ctx, jwksURL,
		jwk.WithMinInterval(refreshInterval),
		jwk.WithWaitReady(true),
	); err != nil {
		return nil, fmt.Errorf("auth: register jwks url: %w", err)
	}
	return &Verifier{
		issuer:    issuer,
		jwksURL:   jwksURL,
		jwksCache: cache,
		queries:   queries,
		redis:     rdb,
	}, nil
}

// VerifyToken verifies a raw bearer token and resolves it to an
// internal user UUID. Pure: no http.Request, no http.ResponseWriter.
// Used by the chi middleware and the Huma middleware adapter.
func (v *Verifier) VerifyToken(ctx context.Context, rawToken string) (uuid.UUID, error) {
	if rawToken == "" {
		return uuid.Nil, errors.New("missing Bearer token")
	}

	keyset, err := v.jwksCache.Lookup(ctx, v.jwksURL)
	if err != nil {
		return uuid.Nil, fmt.Errorf("jwks unavailable: %w", err)
	}

	tok, err := jwt.Parse([]byte(rawToken),
		jwt.WithKeySet(keyset),
		jwt.WithIssuer(v.issuer),
		jwt.WithValidate(true),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}

	var clerkUserID string
	if err := tok.Get("sub", &clerkUserID); err != nil || clerkUserID == "" {
		return uuid.Nil, errors.New("token missing subject")
	}

	// Best-effort email read. Only present if the Clerk session template
	// was extended with `{{user.primary_email_address}}`; otherwise we
	// fall back to a placeholder during lazy provision.
	var emailHint string
	_ = tok.Get("email", &emailHint)

	return v.resolveUserID(ctx, clerkUserID, emailHint)
}

// Middleware is a chi/stdlib http.Handler middleware that enforces
// Clerk auth. Failures return 401 with a JSON error body.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := bearerFromHeader(r.Header.Get(headerAuth))
		userID, err := v.VerifyToken(r.Context(), rawToken)
		if err != nil {
			pkglogger.FromContext(r.Context()).Debug("auth rejected", "err", err)
			writeUnauthorized(w, err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), userIDCtxKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HumaMiddleware adapts the verifier to a Huma middleware. Apply via
// `huma.NewGroup(api).UseMiddleware(verifier.HumaMiddleware(api))`.
// Failures use Huma's WriteErr so the response shape matches the spec.
func (v *Verifier) HumaMiddleware(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		rawToken := bearerFromHeader(ctx.Header(headerAuth))
		userID, err := v.VerifyToken(ctx.Context(), rawToken)
		if err != nil {
			pkglogger.FromContext(ctx.Context()).Debug("auth rejected", "err", err)
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, err.Error())
			return
		}
		next(huma.WithValue(ctx, userIDCtxKey{}, userID))
	}
}

// resolveUserID maps clerk_user_id → internal user UUID. Cache-aside Redis.
// On miss in the DB we lazy-provision a row so Clerk-authed requests don't
// require the webhook to have run first (the webhook is still the canonical
// path; this is the belt for cases where it hasn't fired yet — local dev
// without the webhook URL configured, signups during a webhook outage, etc).
func (v *Verifier) resolveUserID(ctx context.Context, clerkUserID, emailHint string) (uuid.UUID, error) {
	cacheKey := cacheKeyPrefix + clerkUserID

	if cached, err := v.redis.Get(ctx, cacheKey).Result(); err == nil {
		if id, err := uuid.Parse(cached); err == nil {
			return id, nil
		}
		// Bad cache value — let the DB lookup overwrite it.
	}

	user, err := v.queries.GetUserByClerkID(ctx, clerkUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		email := emailHint
		if email == "" {
			email = clerkUserID + "@dev.local"
		}
		user, err = v.queries.UpsertUserByClerkID(ctx, dbgen.UpsertUserByClerkIDParams{
			ClerkUserID: clerkUserID,
			Email:       email,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("user lazy-provision: %w", err)
		}
		pkglogger.FromContext(ctx).Info("lazy-provisioned user",
			"clerk_user_id", clerkUserID, "email", email, "user_id", user.ID)
	} else if err != nil {
		return uuid.Nil, fmt.Errorf("user lookup: %w", err)
	}

	// Best-effort cache write.
	_ = v.redis.Set(ctx, cacheKey, user.ID.String(), cacheTTL).Err()

	return user.ID, nil
}

// UserID returns the internal user UUID stored on ctx by Middleware.
// Second return is false if the request didn't go through Middleware.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(userIDCtxKey{}).(uuid.UUID)
	return v, ok
}

func bearerFromHeader(h string) string {
	if !strings.HasPrefix(h, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	body, _ := json.Marshal(map[string]any{
		"error": map[string]string{"code": "UNAUTHORIZED", "message": msg},
	})
	_, _ = w.Write(body)
}

