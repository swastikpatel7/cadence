// Package onboarding implements the two onboarding endpoints:
//
//   - POST /v1/me/onboarding/complete   (Huma JSON; handlers.go)
//   - GET  /v1/me/onboarding/stream     (SSE, raw chi; sse.go)
//
// SSE handler design (api.md §3): polls river_job every 500ms for the
// user's two onboarding job ids (baseline_compute + plan_initial),
// emits one `step` event per state transition, and `done` when both
// jobs are in a terminal state. Closes on r.Context().Done() (tab
// close) or natural completion.
//
// Idempotency / retry semantics for POST /v1/me/onboarding/complete
// (api.md §1.7 and §8.1 item 5):
//
//   - The user_goals UPSERT is idempotent on user_id.
//   - River jobs are inserted with `UniqueOpts{ByArgs:true,
//     ByPeriod:5m}` so a retry within 5m returns the existing job ids.
//   - Beyond 5m we use LookupOrEnqueueLatest to find the most recent
//     terminal job within 24h before inserting a new one — this
//     prevents a re-clicking user from triggering a second baseline
//     compute the next day.
package onboarding
