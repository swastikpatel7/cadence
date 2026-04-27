-- name: GetGoalByUserID :one
SELECT * FROM user_goals WHERE user_id = $1;

-- name: UpsertGoal :one
-- Used by the onboarding-complete handler. Idempotent on user_id so a
-- retry of POST /v1/me/onboarding/complete doesn't double-insert.
INSERT INTO user_goals (
    user_id, focus, weekly_miles_target, days_per_week,
    target_distance_km, target_pace_sec_per_km, race_date
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id) DO UPDATE
SET focus                  = EXCLUDED.focus,
    weekly_miles_target    = EXCLUDED.weekly_miles_target,
    days_per_week          = EXCLUDED.days_per_week,
    target_distance_km     = EXCLUDED.target_distance_km,
    target_pace_sec_per_km = EXCLUDED.target_pace_sec_per_km,
    race_date              = EXCLUDED.race_date,
    updated_at             = now()
RETURNING *;

-- name: UpdateGoalPartial :one
-- Used by PATCH /v1/me/goal. COALESCE pattern keeps untouched columns.
-- Pass NULLs for fields you do NOT want to change. Explicit-clear of the
-- nullable columns (target_distance_km, target_pace_sec_per_km, race_date)
-- is impossible through this query alone; the handler runs ClearGoalNullable
-- in the same transaction when it sees JSON null in the patch body.
UPDATE user_goals
SET focus                  = COALESCE(sqlc.narg('focus'),                  focus),
    weekly_miles_target    = COALESCE(sqlc.narg('weekly_miles_target'),    weekly_miles_target),
    days_per_week          = COALESCE(sqlc.narg('days_per_week'),          days_per_week),
    target_distance_km     = COALESCE(sqlc.narg('target_distance_km'),     target_distance_km),
    target_pace_sec_per_km = COALESCE(sqlc.narg('target_pace_sec_per_km'), target_pace_sec_per_km),
    race_date              = COALESCE(sqlc.narg('race_date'),              race_date),
    updated_at             = now()
WHERE user_id = sqlc.arg('user_id')
RETURNING *;

-- name: ClearGoalNullable :exec
-- Companion to UpdateGoalPartial: explicitly NULLs the optional fields
-- the patch body sent as JSON null. The handler runs both queries in a
-- single transaction.
UPDATE user_goals
SET target_distance_km     = CASE WHEN sqlc.arg('clear_distance')::bool THEN NULL ELSE target_distance_km END,
    target_pace_sec_per_km = CASE WHEN sqlc.arg('clear_pace')::bool     THEN NULL ELSE target_pace_sec_per_km END,
    race_date              = CASE WHEN sqlc.arg('clear_race_date')::bool THEN NULL ELSE race_date END,
    updated_at             = now()
WHERE user_id = sqlc.arg('user_id');
