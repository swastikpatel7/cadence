// Package jobs holds River job definitions. Each job is one type
// implementing river.JobArgs (for the args struct) plus one type
// implementing river.Worker[Args] (for the handler).
//
// Phase 1 ships a noop job to prove the loop works. Phase 4 replaces
// this file with strava_sync, strava_backfill, and
// strava_token_refresh.
package jobs

import (
	"context"

	"github.com/riverqueue/river"

	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// NoopArgs is a placeholder job that does nothing. Used by Phase 1 to
// verify River is wired and processing.
type NoopArgs struct{}

// Kind is River's job-type discriminator.
func (NoopArgs) Kind() string { return "noop" }

// NoopWorker is the handler for NoopArgs.
type NoopWorker struct {
	river.WorkerDefaults[NoopArgs]
}

// Work logs that the job ran and returns nil.
func (w *NoopWorker) Work(ctx context.Context, j *river.Job[NoopArgs]) error {
	pkglogger.FromContext(ctx).Info("noop job ran",
		"job_id", j.ID,
		"attempt", j.Attempt,
	)
	return nil
}
