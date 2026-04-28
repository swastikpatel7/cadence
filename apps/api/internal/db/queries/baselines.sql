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
-- Backs GET /v1/me/baseline. Partial index `baselines_user_recent`
-- (WHERE archived_at IS NULL) covers this.
SELECT * FROM baselines
WHERE user_id = $1 AND archived_at IS NULL
ORDER BY computed_at DESC
LIMIT 1;

-- name: ListBaselinesByUserID :many
-- For the audit / history view (deferred to v2; cheap to ship now).
SELECT * FROM baselines
WHERE user_id = $1 AND archived_at IS NULL
ORDER BY computed_at DESC
LIMIT $2;

-- name: ArchiveBaselinesByUserID :exec
-- Used by POST /v1/me/onboarding/reset. Soft-deletes all of a user's
-- baselines so a fresh wizard run starts with no history visible. The
-- archived rows remain in-table for cost auditing.
UPDATE baselines SET archived_at = now()
 WHERE user_id = $1 AND archived_at IS NULL;
