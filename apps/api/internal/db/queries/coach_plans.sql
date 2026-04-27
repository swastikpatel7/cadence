-- name: InsertPlan :one
-- Used by InitialPlanWorker (generation_kind='initial_8wk') and
-- WeeklyRefreshWorker (generation_kind='weekly_refresh'). plan jsonb is
-- the structured {weeks:[{week_index,total_km,sessions:[...]}]} blob.
INSERT INTO coach_plans (
    user_id, generation_kind, baseline_id, goal_id,
    starts_on, weeks_count, plan,
    model, input_tokens, output_tokens, thinking_tokens, reason
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetCurrentPlanByUserID :one
-- The "live" plan is the one with superseded_by IS NULL. Index
-- `coach_plans_user_current` is partial-on-that-condition.
SELECT * FROM coach_plans
WHERE user_id = $1 AND superseded_by IS NULL
ORDER BY starts_on DESC
LIMIT 1;

-- name: GetPlanWindow :many
-- Used by the heatmap endpoint to get the current plan that overlaps
-- [window_start, window_end]. We pull the current plan only — the
-- frontend stitches per-cell from its `plan` JSONB.
SELECT * FROM coach_plans
WHERE user_id = $1
  AND superseded_by IS NULL
  AND starts_on <= $3
  AND (starts_on + (weeks_count || ' weeks')::interval)::date >= $2;

-- name: MarkPlanSuperseded :exec
-- Called after a successful WeeklyRefreshWorker INSERT to chain the
-- prior current plan to the new one.
UPDATE coach_plans
SET superseded_by = $2
WHERE user_id = $1
  AND superseded_by IS NULL
  AND id <> $2;
