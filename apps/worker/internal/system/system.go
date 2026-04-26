// Package system holds the worker's composition root. Mirrors
// apps/api/internal/system but wires River instead of an HTTP server.
package system

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/swastikpatel7/cadence/apps/worker/internal/config"
	"github.com/swastikpatel7/cadence/apps/worker/internal/jobs"
	pkgcrypto "github.com/swastikpatel7/cadence/pkg/crypto"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
	"github.com/swastikpatel7/cadence/pkg/strava"
)

// Dependencies is the live wiring of the worker.
type Dependencies struct {
	Logger *slog.Logger
	Config *config.Config
	DB     *pgxpool.Pool
	River  *river.Client[pgx.Tx]
}

// InitDependencies wires logger + config + Postgres pool + River client.
// Caller must call Close to release resources.
func InitDependencies(ctx context.Context) (*Dependencies, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("system: load config: %w", err)
	}

	log := pkglogger.New(pkglogger.Env(cfg.AppEnv), cfg.LogLevel)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("system: connect postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: ping postgres: %w", err)
	}

	queries := dbgen.New(pool)

	keyBytes, err := base64.StdEncoding.DecodeString(cfg.EncryptionKey)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: decode encryption key: %w", err)
	}
	cipher, err := pkgcrypto.NewTokenCipher(keyBytes)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: token cipher: %w", err)
	}
	// RedirectURL is unused for refreshes; leave empty here.
	oauthCfg := strava.NewOAuthConfig(cfg.StravaClientID, cfg.StravaClientSecret, "")

	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.NoopWorker{})
	river.AddWorker(workers, jobs.NewStravaSyncWorker(pool, queries, cipher, oauthCfg))

	periodicJobs := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(30*time.Second),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.NoopArgs{}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: true},
		),
	}

	rc, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 5},
		},
		Workers:      workers,
		PeriodicJobs: periodicJobs,
		Logger:       log,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("system: new river client: %w", err)
	}

	return &Dependencies{
		Logger: log,
		Config: cfg,
		DB:     pool,
		River:  rc,
	}, nil
}

// Close releases the Postgres pool. River is stopped separately by main
// (via Stop with a drain context).
func (d *Dependencies) Close() error {
	if d.DB != nil {
		d.DB.Close()
	}
	return nil
}
