// Package system holds the composition root: a single InitDependencies
// function that wires all components together, returning a Dependencies
// struct that exposes only what main needs.
//
// Per the Cadence playbook (Patterns/Dependency Injection): one
// composition root, constructor injection, no init() globals.
package system

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	"github.com/swastikpatel7/cadence/apps/api/internal/config"
	"github.com/swastikpatel7/cadence/apps/api/internal/connections"
	"github.com/swastikpatel7/cadence/apps/api/internal/server"
	"github.com/swastikpatel7/cadence/apps/api/internal/server/handlers"
	pkgcrypto "github.com/swastikpatel7/cadence/pkg/crypto"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
	"github.com/swastikpatel7/cadence/pkg/strava"
)

const (
	startupPingTimeout = 5 * time.Second
	readyPingTimeout   = 2 * time.Second
)

// Dependencies is the live wiring of the API service. main never sees
// internal clients (DB pool, Redis) directly — those stay private and
// are consumed by services/handlers via constructor injection.
type Dependencies struct {
	Logger      *slog.Logger
	Config      *config.Config
	DB          *pgxpool.Pool
	Redis       *redis.Client
	Queries     *dbgen.Queries
	Verifier    *auth.Verifier
	River       *river.Client[pgx.Tx]
	Connections *connections.Service
	Server      *http.Server
}

// InitDependencies wires logger + config + DB + Redis + Clerk verifier
// + HTTP server. Fails fast if Postgres or Redis aren't reachable.
// Clerk verifier is optional in dev (skipped if CLERK_JWKS_URL is empty).
func InitDependencies(ctx context.Context) (*Dependencies, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("system: load config: %w", err)
	}

	log := pkglogger.New(pkglogger.Env(cfg.AppEnv), cfg.LogLevel)

	pool, err := newPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("system: postgres: %w", err)
	}

	rdb, err := newRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: redis: %w", err)
	}

	queries := dbgen.New(pool)

	var verifier *auth.Verifier
	if cfg.ClerkJWKSURL != "" {
		verifier, err = auth.NewVerifier(ctx, cfg.ClerkJWKSURL, cfg.ClerkIssuer, queries, rdb)
		if err != nil {
			pool.Close()
			_ = rdb.Close()
			return nil, fmt.Errorf("system: clerk verifier: %w", err)
		}
	} else {
		log.Warn("CLERK_JWKS_URL not set; authed routes disabled (dev only)")
	}

	// Strava connections: only wired if all required env vars are present.
	// In dev without Strava credentials, the API still starts; the start /
	// callback / sync endpoints are simply not registered.
	var (
		connService *connections.Service
		riverClient *river.Client[pgx.Tx]
	)
	if hasStravaConfig(cfg) {
		cipher, err := newTokenCipher(cfg.EncryptionKey)
		if err != nil {
			pool.Close()
			_ = rdb.Close()
			return nil, fmt.Errorf("system: token cipher: %w", err)
		}
		// Insert-only River client: never Start()ed. River still requires
		// every kind we Insert() to be present in the Workers bundle, so
		// pkg/strava.RegisterInsertOnlyWorkers seeds it with stubs for each
		// Strava-side job kind. The real processing happens in apps/worker.
		insertWorkers := river.NewWorkers()
		strava.RegisterInsertOnlyWorkers(insertWorkers)
		riverClient, err = river.NewClient(riverpgxv5.New(pool), &river.Config{
			Workers: insertWorkers,
			Logger:  log,
		})
		if err != nil {
			pool.Close()
			_ = rdb.Close()
			return nil, fmt.Errorf("system: river client: %w", err)
		}
		oauthCfg := strava.NewOAuthConfig(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaCallbackURL)
		connService = connections.NewService(connections.Deps{
			Queries: queries,
			State:   connections.NewStateStore(rdb),
			Cipher:  cipher,
			OAuth:   oauthCfg,
			River:   riverClient,
			Logger:  log,
		})
	} else {
		log.Warn("Strava env vars incomplete; connection endpoints disabled (dev only)")
	}

	srv, err := server.New(server.Deps{
		Config:        cfg,
		Logger:        log,
		Queries:       queries,
		Verifier:      verifier,
		ReadyProbe:    makeReadyProbe(pool, rdb),
		WebhookSecret: cfg.ClerkWebhookSecret,
		Connections:   connService,
		WebBaseURL:    cfg.WebBaseURL,
	})
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("system: build server: %w", err)
	}

	return &Dependencies{
		Logger:      log,
		Config:      cfg,
		DB:          pool,
		Redis:       rdb,
		Queries:     queries,
		Verifier:    verifier,
		River:       riverClient,
		Connections: connService,
		Server:      srv,
	}, nil
}

// hasStravaConfig reports whether the env has everything we need to wire
// the Strava OAuth + sync stack. Missing pieces leave the feature off.
func hasStravaConfig(cfg *config.Config) bool {
	return cfg.StravaClientID != "" &&
		cfg.StravaClientSecret != "" &&
		cfg.StravaCallbackURL != "" &&
		cfg.EncryptionKey != ""
}

// newTokenCipher decodes the base64 ENCRYPTION_KEY and constructs the
// AES-256-GCM cipher we use for at-rest token storage.
func newTokenCipher(b64Key string) (*pkgcrypto.TokenCipher, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	return pkgcrypto.NewTokenCipher(keyBytes)
}

func newPostgresPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, startupPingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func newRedisClient(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, startupPingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return client, nil
}

func makeReadyProbe(pool *pgxpool.Pool, rdb *redis.Client) handlers.ReadyProbe {
	return func(ctx context.Context) error {
		pingCtx, cancel := context.WithTimeout(ctx, readyPingTimeout)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			return fmt.Errorf("redis: %w", err)
		}
		return nil
	}
}

// Close releases pool and redis client. Safe to call multiple times.
func (d *Dependencies) Close() error {
	var errs []error
	if d.Redis != nil {
		if err := d.Redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("redis close: %w", err))
		}
	}
	if d.DB != nil {
		d.DB.Close()
	}
	return errors.Join(errs...)
}
