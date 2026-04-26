package connections

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	stateKeyPrefix = "strava:oauth:"
	stateTTL       = 5 * time.Minute
	stateRandBytes = 32
)

// StateStore is the OAuth state-token CSRF guard for the Strava
// connect flow. Each /start call generates a random state, binds it to
// the calling user UUID in Redis, and the /callback consumes that
// binding (GETDEL) before exchanging the code.
//
// Because the binding is consumed exactly once, replay attacks against
// the callback URL fail.
type StateStore struct {
	rdb *redis.Client
}

// NewStateStore constructs a StateStore.
func NewStateStore(rdb *redis.Client) *StateStore { return &StateStore{rdb: rdb} }

// Generate produces a fresh state token bound to userID. Caller embeds
// the returned string in the authorize URL's `state` query param.
func (s *StateStore) Generate(ctx context.Context, userID uuid.UUID) (string, error) {
	buf := make([]byte, stateRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("oauth state: rand: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)
	if err := s.rdb.Set(ctx, stateKeyPrefix+state, userID.String(), stateTTL).Err(); err != nil {
		return "", fmt.Errorf("oauth state: store: %w", err)
	}
	return state, nil
}

// Consume validates and atomically removes a state token. Returns the
// userID it was bound to, or an error if the token is unknown or expired.
func (s *StateStore) Consume(ctx context.Context, state string) (uuid.UUID, error) {
	if state == "" {
		return uuid.Nil, errors.New("oauth state: empty")
	}
	v, err := s.rdb.GetDel(ctx, stateKeyPrefix+state).Result()
	if err == redis.Nil {
		return uuid.Nil, errors.New("oauth state: unknown or expired")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("oauth state: load: %w", err)
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, fmt.Errorf("oauth state: bound id invalid: %w", err)
	}
	return id, nil
}
