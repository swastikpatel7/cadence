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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"golang.org/x/oauth2"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	"github.com/swastikpatel7/cadence/apps/api/internal/baseline"
	"github.com/swastikpatel7/cadence/apps/api/internal/cache"
	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
	"github.com/swastikpatel7/cadence/apps/api/internal/config"
	"github.com/swastikpatel7/cadence/apps/api/internal/connections"
	"github.com/swastikpatel7/cadence/apps/api/internal/jobs"
	"github.com/swastikpatel7/cadence/apps/api/internal/plan"
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

// Dependencies is the live wiring of the API service. The same process
// owns the HTTP server and the River worker pool; main starts both and
// drains both on shutdown.
type Dependencies struct {
	Logger      *slog.Logger
	Config      *config.Config
	DB          *pgxpool.Pool
	Queries     *dbgen.Queries
	Verifier    *auth.Verifier
	River       *river.Client[pgx.Tx]
	Connections *connections.Service
	Coach       *coach.Client
	Server      *http.Server
}

// InitDependencies wires logger + config + DB + caches + Clerk verifier
// + River + HTTP server. Fails fast if Postgres isn't reachable.
//
// Optional pieces:
//   - Clerk verifier (skipped if CLERK_JWKS_URL is empty)
//   - Strava connections + sync worker (skipped if Strava env is incomplete)
//
// River is always started: even without Strava, the noop periodic job
// runs as a sanity check that the queue loop is healthy.
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

	if err := migrateRiverSchema(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: river migrate: %w", err)
	}

	queries := dbgen.New(pool)

	userCache := cache.New[uuid.UUID]()

	var verifier *auth.Verifier
	if cfg.ClerkJWKSURL != "" {
		verifier, err = auth.NewVerifier(ctx, cfg.ClerkJWKSURL, cfg.ClerkIssuer, queries, userCache)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("system: clerk verifier: %w", err)
		}
	} else {
		log.Warn("CLERK_JWKS_URL not set; authed routes disabled (dev only)")
	}

	// Strava-side wiring (cipher + OAuth + sync worker). All-or-nothing:
	// if any required env var is missing, the connections feature stays
	// off and the sync worker is not registered.
	var (
		cipher    *pkgcrypto.TokenCipher
		oauthCfg  *oauth2.Config
		hasStrava = hasStravaConfig(cfg)
	)
	if hasStrava {
		cipher, err = newTokenCipher(cfg.EncryptionKey)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("system: token cipher: %w", err)
		}
		oauthCfg = strava.NewOAuthConfig(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaCallbackURL)
	} else {
		log.Warn("Strava env vars incomplete; connection endpoints + sync worker disabled (dev only)")
	}

	// Coach: shared Anthropic client for baseline/plan/micro-summary
	// workers. Optional — if ANTHROPIC_API_KEY is empty the workers
	// register normally but log + JobSnooze when invoked.
	coachClient := coach.New(coach.Config{
		APIKey:      cfg.AnthropicAPIKey,
		Logger:      log,
		ModelOpus:   cfg.AnthropicModelOpus,
		ModelSonnet: cfg.AnthropicModelSonnet,
		ModelHaiku:  cfg.AnthropicModelHaiku,
		BaseURL:     cfg.CoachAIGatewayBaseURL,
	})

	// Build the worker bundle before the client (river requires every
	// kind we Insert() to be registered up front).
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.NoopWorker{})
	if hasStrava {
		river.AddWorker(workers, jobs.NewStravaSyncWorker(pool, queries, cipher, oauthCfg))
	}
	// Plan workers: registered before the client so the periodic-cron
	// kind matches (River asserts at startup).
	initialPlanWorker := plan.NewInitialWorker(queries, coachClient)
	weeklyRefreshWorker := plan.NewWeeklyRefreshWorker(pool, queries, coachClient)
	microSummaryWorker := plan.NewSessionMicroSummaryWorker(queries, coachClient)
	river.AddWorker(workers, initialPlanWorker)
	river.AddWorker(workers, weeklyRefreshWorker)
	river.AddWorker(workers, microSummaryWorker)

	periodicJobs := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(30*time.Second),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.NoopArgs{}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: true},
		),
		jobs.NewWeeklyRefreshPeriodic(),
	}

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers:      workers,
		PeriodicJobs: periodicJobs,
		Logger:       log,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: river client: %w", err)
	}

	// BaselineComputeWorker is special: it both consumes JobArgs and
	// chains an InitialPlan job via the river client. We add it after
	// constructing the client so the dependency cycle resolves.
	baselineWorker := baseline.NewWorker(pool, queries, coachClient, riverClient)
	river.AddWorker(workers, baselineWorker)

	var connService *connections.Service
	if hasStrava {
		connService = connections.NewService(connections.Deps{
			Queries: queries,
			State:   connections.NewStateStore(cache.New[uuid.UUID]()),
			Cipher:  cipher,
			OAuth:   oauthCfg,
			River:   riverClient,
			Logger:  log,
		})
	}

	srv, err := server.New(server.Deps{
		Config:        cfg,
		Logger:        log,
		Queries:       queries,
		Verifier:      verifier,
		ReadyProbe:    makeReadyProbe(pool),
		WebhookSecret: cfg.ClerkWebhookSecret,
		Connections:   connService,
		WebBaseURL:    cfg.WebBaseURL,
		DB:            pool,
		River:         riverClient,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: build server: %w", err)
	}

	// Reference the workers we hold so go vet doesn't flag them as
	// unused. River.AddWorker takes them by value-pointer; the locals
	// are kept primarily so we can attach hooks later.
	_ = initialPlanWorker
	_ = weeklyRefreshWorker
	_ = microSummaryWorker
	_ = baselineWorker

	return &Dependencies{
		Logger:      log,
		Config:      cfg,
		DB:          pool,
		Queries:     queries,
		Verifier:    verifier,
		River:       riverClient,
		Connections: connService,
		Coach:       coachClient,
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

// migrateRiverSchema applies River's internal migrations (river_job,
// river_queue, river_leader, river_client, river_migration). Idempotent:
// River tracks applied versions in river_migration, so reruns are no-ops.
// Goose owns the application schema only; River owns its own.
func migrateRiverSchema(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	return nil
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

func makeReadyProbe(pool *pgxpool.Pool) handlers.ReadyProbe {
	return func(ctx context.Context) error {
		pingCtx, cancel := context.WithTimeout(ctx, readyPingTimeout)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		return nil
	}
}

// Close releases the Postgres pool. River is stopped separately by main
// (via Stop with a drain context).
func (d *Dependencies) Close() error {
	var errs []error
	if d.DB != nil {
		d.DB.Close()
	}
	return errors.Join(errs...)
}
