// Package plan implements the plan/heatmap slice of the
// onboarding/baseline/plan stack:
//
//   - GET  /v1/me/plan/heatmap          (handlers.go)
//   - GET  /v1/me/plan/session/:date    (handlers.go)
//   - POST /v1/me/plan/refresh          (handlers.go)
//   - InitialPlanWorker                 (worker.go)
//   - WeeklyRefreshWorker               (worker.go)
//   - SessionMicroSummaryWorker         (worker.go)
//
// Three workers, three Anthropic models:
//
//   - InitialPlanWorker          — Opus 4.7,    one-time 8-week plan
//   - WeeklyRefreshWorker        — Sonnet 4.6,  cron + manual + goal_change
//   - SessionMicroSummaryWorker  — Haiku 4.5,   lazy on heatmap drawer open
//
// Heatmap join logic lives in heatmap.go and is the only place that
// stitches `coach_plans.plan` JSONB + `activities` rows into wire-shape
// cells. Per api.md §8.2 we deliberately do NOT join `coach_insights`
// here — micro-summaries only enter via the session-detail endpoint.
package plan
