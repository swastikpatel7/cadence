-- name: InsertBaseline :one
-- Inserted by BaselineComputeWorker after the Opus 4.7 narrative call.
-- avg_pace_at_distance is a JSONB blob keyed by distance-int → sec/km.
INSERT INTO baselines (
    user_id, window_days, fitness_tier,
    weekly_volume_km_avg, weekly_volume_km_p25, weekly_volume_km_p75,
    avg_pace_sec_per_km, avg_pace_at_distance, longest_run_km,
    consistency_score, narrative, source,
    model, input_tokens, output_tokens, thinking_tokens
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetLatestBaselineByUserID :one
-- Backs GET /v1/me/baseline. Index `baselines_user_recent` covers this.
SELECT * FROM baselines
WHERE user_id = $1
ORDER BY computed_at DESC
LIMIT 1;

-- name: ListBaselinesByUserID :many
-- For the audit / history view (deferred to v2; cheap to ship now).
SELECT * FROM baselines
WHERE user_id = $1
ORDER BY computed_at DESC
LIMIT $2;
