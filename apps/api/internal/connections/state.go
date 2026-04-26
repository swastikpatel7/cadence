package connections

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/swastikpatel7/cadence/apps/api/internal/cache"
)

const (
	stateTTL       = 5 * time.Minute
	stateRandBytes = 32
)

// StateStore is the OAuth state-token CSRF guard for the Strava
// connect flow. Each /start call generates a random state, binds it to
// the calling user UUID in an in-process TTL cache, and the /callback
// consumes that binding (Take = atomic GetDelete) before exchanging
// the code.
//
// Because the binding is consumed exactly once, replay attacks against
// the callback URL fail.
//
// Single-instance assumption: state lives in memory. If the API is ever
// scaled horizontally, swap the cache for a Postgres-backed store.
type StateStore struct {
	c *cache.Cache[uuid.UUID]
}

// NewStateStore constructs a StateStore.
func NewStateStore(c *cache.Cache[uuid.UUID]) *StateStore { return &StateStore{c: c} }

// Generate produces a fresh state token bound to userID. Caller embeds
// the returned string in the authorize URL's `state` query param.
func (s *StateStore) Generate(_ context.Context, userID uuid.UUID) (string, error) {
	buf := make([]byte, stateRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("oauth state: rand: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)
	s.c.Set(state, userID, stateTTL)
	return state, nil
}

// Consume validates and atomically removes a state token. Returns the
// userID it was bound to, or an error if the token is unknown or expired.
func (s *StateStore) Consume(_ context.Context, state string) (uuid.UUID, error) {
	if state == "" {
		return uuid.Nil, errors.New("oauth state: empty")
	}
	id, ok := s.c.Take(state)
	if !ok {
		return uuid.Nil, errors.New("oauth state: unknown or expired")
	}
	return id, nil
}
