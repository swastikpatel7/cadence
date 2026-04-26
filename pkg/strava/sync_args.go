package strava

import "github.com/google/uuid"

// SyncJobKind is the River job-type discriminator. Lives next to the
// args struct so anything inserting jobs (apps/api) and the worker
// itself (apps/worker) reference the same string.
const SyncJobKind = "strava_sync"

// SyncJobArgs is the River JobArgs payload for a manual Strava sync.
//
// AfterTs is the lower bound on activity start_date (unix seconds) for
// the initial pass. Once the job is running, the resume cursor lives
// in connected_accounts.sync_progress (so a JobSnooze on 429 can
// continue from the last activity processed without re-listing).
type SyncJobArgs struct {
	UserID  uuid.UUID `json:"user_id"`
	AfterTs int64     `json:"after_ts"`
}

// Kind satisfies river.JobArgs without importing river here (which keeps
// this package buildable from apps/api which doesn't actually run a
// worker).
func (SyncJobArgs) Kind() string { return SyncJobKind }
