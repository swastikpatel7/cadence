-- name: GetInsightForActivity :one
-- Used by the session-detail handler to read the cached micro-summary.
-- Filters out archived rows from a prior onboarding cycle.
SELECT * FROM coach_insights
WHERE activity_id = $1 AND kind = $2 AND archived_at IS NULL;

-- name: InsertInsight :one
-- Used by SessionMicroSummaryWorker. Idempotent on the *active*
-- (activity_id, kind) pair. The migration rebuilt the UNIQUE index as
-- partial on `WHERE archived_at IS NULL`, so the ON CONFLICT clause
-- must echo that predicate to find the partial index.
INSERT INTO coach_insights (user_id, activity_id, kind, body, model, input_tokens, output_tokens)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (activity_id, kind) WHERE archived_at IS NULL DO UPDATE
SET body          = EXCLUDED.body,
    model         = EXCLUDED.model,
    input_tokens  = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    created_at    = now()
RETURNING *;

-- name: ArchiveCoachInsightsByUserID :exec
-- Used by POST /v1/me/onboarding/reset. Soft-deletes all of a user's
-- micro-summaries. After reset, the next session-detail open will
-- regenerate via Haiku (or hit the partial-unique safely beside the
-- archived row).
UPDATE coach_insights SET archived_at = now()
 WHERE user_id = $1 AND archived_at IS NULL;
