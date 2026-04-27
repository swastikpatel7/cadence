package baseline

import "github.com/google/uuid"

// JobKind is the River discriminator. Lives next to JobArgs so handlers
// (which insert) and workers (which dispatch on Kind) share one source
// of truth.
const JobKind = "baseline_compute"

// Trigger* values are the legal `Trigger` field values on JobArgs.
// They map directly to the `source` column on the `baselines` table.
const (
	TriggerOnboarding      = "onboarding"
	TriggerManualRecompute = "manual_recompute"
	TriggerSyncMilestone   = "sync_milestone"
)

// JobArgs is the River JobArgs payload for BaselineComputeWorker.
//
// Window is the lookback in days over which the volume curve is
// computed. -1 means "all history" — the worker translates that to a
// no-bounds query.
//
// Trigger differentiates the three callsites and lands on the
// `baselines.source` column.
type JobArgs struct {
	UserID  uuid.UUID `json:"user_id"`
	Window  int       `json:"window"`
	Trigger string    `json:"trigger"`
}

// Kind satisfies river.JobArgs.
func (JobArgs) Kind() string { return JobKind }
