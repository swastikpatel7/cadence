-- name: GetInsightForActivity :one
-- Used by the session-detail handler to read the cached micro-summary.
SELECT * FROM coach_insights
WHERE activity_id = $1 AND kind = $2;

-- name: InsertInsight :one
-- Used by SessionMicroSummaryWorker. Idempotent on (activity_id, kind).
INSERT INTO coach_insights (user_id, activity_id, kind, body, model, input_tokens, output_tokens)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (activity_id, kind) DO UPDATE
SET body          = EXCLUDED.body,
    model         = EXCLUDED.model,
    input_tokens  = EXCLUDED.input_tokens,
    output_tokens = EXCLUDED.output_tokens,
    created_at    = now()
RETURNING *;
