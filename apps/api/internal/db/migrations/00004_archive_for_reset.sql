-- +goose Up
-- +goose StatementBegin

-- Soft-delete column for the "reset onboarding" flow. A non-null
-- archived_at hides the row from active reads but preserves it for
-- cost auditing and possible undo.
ALTER TABLE baselines      ADD COLUMN archived_at timestamptz;
ALTER TABLE coach_plans    ADD COLUMN archived_at timestamptz;
ALTER TABLE coach_insights ADD COLUMN archived_at timestamptz;

-- Replace baselines_user_recent with a partial index over active rows
-- only. The "latest baseline" lookup is the hot path; we don't want it
-- to scan archived history.
DROP INDEX IF EXISTS baselines_user_recent;
CREATE INDEX baselines_user_recent
    ON baselines (user_id, computed_at DESC)
    WHERE archived_at IS NULL;

-- Tighten the live-plan partial — a plan must be both un-superseded AND
-- un-archived to count as "current".
DROP INDEX IF EXISTS coach_plans_user_current;
CREATE INDEX coach_plans_user_current
    ON coach_plans (user_id, starts_on)
    WHERE superseded_by IS NULL AND archived_at IS NULL;

-- Rebuild the (activity_id, kind) UNIQUE as a partial. Without this, an
-- archived row from a previous onboarding cycle would block a fresh
-- post-reset insight insert for the same activity (UNIQUE collision).
-- A partial unique lets archived + active coexist.
DROP INDEX IF EXISTS coach_insights_activity_kind;
CREATE UNIQUE INDEX coach_insights_activity_kind
    ON coach_insights (activity_id, kind)
    WHERE archived_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS coach_insights_activity_kind;
CREATE UNIQUE INDEX coach_insights_activity_kind
    ON coach_insights (activity_id, kind);

DROP INDEX IF EXISTS coach_plans_user_current;
CREATE INDEX coach_plans_user_current
    ON coach_plans (user_id, starts_on)
    WHERE superseded_by IS NULL;

DROP INDEX IF EXISTS baselines_user_recent;
CREATE INDEX baselines_user_recent
    ON baselines (user_id, computed_at DESC);

ALTER TABLE coach_insights DROP COLUMN archived_at;
ALTER TABLE coach_plans    DROP COLUMN archived_at;
ALTER TABLE baselines      DROP COLUMN archived_at;

-- +goose StatementEnd
