// Package baseline implements the baseline-compute slice of the
// onboarding/baseline/plan stack:
//
//   - GET  /v1/me/baseline           (handlers.go)
//   - POST /v1/me/baseline/recompute (handlers.go)
//   - BaselineComputeWorker          (worker.go)
//
// The worker stages map 1:1 to the SSE step IDs the onboarding handler
// emits (`sync`, `volume_curve`, `baseline`); see api.md §3.2.
//
// Numeric derivation lives in compute.go (pure SQL + Go math, no LLM
// call). The Opus 4.7 narrative call lives in narrate.go and is the
// only Anthropic touchpoint in this package.
//
// Idempotency:
//   - POST /v1/me/baseline/recompute returns 409 if a job is already
//     in `available` or `running` state for this user, by checking the
//     river_job table directly. No insert is attempted in that case
//     so we never accidentally double-bill the API.
//   - The worker itself is safe to retry: each invocation re-reads
//     the latest activity table and re-inserts a fresh `baselines`
//     row (history is preserved by design — see schema.sql).
package baseline
