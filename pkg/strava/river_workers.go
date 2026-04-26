package strava

import (
	"context"
	"errors"

	"github.com/riverqueue/river"
)

// RegisterInsertOnlyWorkers registers stub Workers for every Strava-side
// job kind on the given bundle. The API uses this to satisfy River's
// requirement that any kind passed to Client.Insert() must be present in
// the Workers bundle. The real processing logic lives in apps/worker,
// which builds proper workers with their dependency graph and registers
// them directly.
//
// If the stub ever runs (i.e. someone Start()s the API's River client by
// mistake), Work returns a sentinel error so the misconfiguration is
// loud rather than silent.
//
// When a new Strava-side kind is added, register a stub for it here so
// the API picks it up automatically.
func RegisterInsertOnlyWorkers(workers *river.Workers) {
	river.AddWorker(workers, &insertOnlySyncStub{})
}

type insertOnlySyncStub struct {
	river.WorkerDefaults[SyncJobArgs]
}

var errInsertOnlyStub = errors.New(
	"strava: insert-only stub Worker.Work called — the API's River client should never be Start()ed",
)

func (insertOnlySyncStub) Work(_ context.Context, _ *river.Job[SyncJobArgs]) error {
	return errInsertOnlyStub
}
