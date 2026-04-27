-- +goose Up
-- +goose StatementBegin

-- ─── user_goals ──────────────────────────────────────────────────────
-- One row per user. Insertion happens at end of onboarding wizard;
-- subsequent edits use UPSERT semantics keyed on user_id UNIQUE.
-- focus enum gates plan-generation prompt branches.
-- weekly_miles_target stays in MILES (not km) because the wizard slider
-- is mile-denominated and re-deriving it from a km value would be lossy.
-- target_* and race_date are nullable: only "train_for_race" needs them.
CREATE TABLE user_goals (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  uuid        NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    focus                    text        NOT NULL CHECK (focus IN ('general','build_distance','build_speed','train_for_race')),
    weekly_miles_target      integer     NOT NULL CHECK (weekly_miles_target BETWEEN 5 AND 80),
    days_per_week            integer     NOT NULL CHECK (days_per_week BETWEEN 3 AND 7),
    target_distance_km       numeric(6,2),
    target_pace_sec_per_km   integer,
    race_date                date,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_user_goals_updated_at
    BEFORE UPDATE ON user_goals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ─── baselines ───────────────────────────────────────────────────────
-- Computed baselines. History is preserved (one row per recompute); the
-- "current" baseline is `ORDER BY computed_at DESC LIMIT 1`.
-- avg_pace_at_distance is a JSON map of {distance_km_int: sec_per_km}
-- (e.g. {"5": 294, "10": 319}); kept as jsonb to stay schema-flexible.
-- model + token columns let ops dashboards track Anthropic spend per
-- user; not exposed on the API surface.
CREATE TABLE baselines (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    window_days              integer     NOT NULL,
    computed_at              timestamptz NOT NULL DEFAULT now(),
    fitness_tier             text        NOT NULL CHECK (fitness_tier IN ('T1','T2','T3','T4','T5')),
    weekly_volume_km_avg     numeric(6,2) NOT NULL,
    weekly_volume_km_p25     numeric(6,2) NOT NULL,
    weekly_volume_km_p75     numeric(6,2) NOT NULL,
    avg_pace_sec_per_km      integer     NOT NULL,
    avg_pace_at_distance     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    longest_run_km           numeric(6,2) NOT NULL,
    consistency_score        integer     NOT NULL CHECK (consistency_score BETWEEN 0 AND 100),
    narrative                text        NOT NULL,
    source                   text        NOT NULL CHECK (source IN ('onboarding','manual_recompute','sync_milestone')),
    model                    text        NOT NULL,
    input_tokens             integer     NOT NULL,
    output_tokens            integer     NOT NULL,
    thinking_tokens          integer     NOT NULL DEFAULT 0
);

CREATE INDEX baselines_user_recent
    ON baselines (user_id, computed_at DESC);


-- ─── coach_plans ─────────────────────────────────────────────────────
-- Generated training plans. generation_kind distinguishes the one-time
-- 8-week initial plan (Opus 4.7) from the recurring weekly refresh
-- (Sonnet 4.6). superseded_by chains old → new so the "current" plan
-- is `WHERE superseded_by IS NULL`. plan jsonb holds the structured
-- {weeks:[{week_index,total_km,sessions:[...]}]} blob the heatmap
-- handler iterates over to project per-day cells.
-- reason ∈ ('onboarding','weekly_cron','goal_change','manual') tracks
-- why a refresh fired.
CREATE TABLE coach_plans (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    generation_kind          text        NOT NULL CHECK (generation_kind IN ('initial_8wk','weekly_refresh')),
    baseline_id              uuid        REFERENCES baselines(id) ON DELETE SET NULL,
    goal_id                  uuid        REFERENCES user_goals(id) ON DELETE SET NULL,
    starts_on                date        NOT NULL,
    weeks_count              integer     NOT NULL,
    generated_at             timestamptz NOT NULL DEFAULT now(),
    plan                     jsonb       NOT NULL,
    model                    text        NOT NULL,
    input_tokens             integer     NOT NULL,
    output_tokens            integer     NOT NULL,
    thinking_tokens          integer     NOT NULL DEFAULT 0,
    superseded_by            uuid        REFERENCES coach_plans(id),
    reason                   text
);

-- Partial index over the live plan only — keeps the hot working set tiny.
CREATE INDEX coach_plans_user_current
    ON coach_plans (user_id, starts_on)
    WHERE superseded_by IS NULL;


-- ─── coach_insights ──────────────────────────────────────────────────
-- Originally planned for the coach v1 milestone; folded in here because
-- the session-detail endpoint relies on it for the lazy Haiku micro-
-- summary. body is plain text for v1 (not jsonb) — keeps writes simple.
-- (activity_id, kind) UNIQUE makes the worker insertion idempotent so
-- two concurrent SessionMicroSummaryWorker invocations land on the
-- ON CONFLICT path instead of double-billing Haiku.
CREATE TABLE coach_insights (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    activity_id     uuid        REFERENCES activities(id) ON DELETE CASCADE,
    kind            text        NOT NULL,
    body            text        NOT NULL,
    model           text        NOT NULL,
    input_tokens    integer     NOT NULL DEFAULT 0,
    output_tokens   integer     NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- One insight row per (activity, kind). Lets the worker UPSERT cleanly
-- and lets the read path do `WHERE activity_id = $1 AND kind = $2`.
CREATE UNIQUE INDEX coach_insights_activity_kind
    ON coach_insights (activity_id, kind);

-- Per-user feed lookups (e.g. "show me my recent micro-summaries").
CREATE INDEX coach_insights_user_recent
    ON coach_insights (user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS coach_insights;
DROP TABLE IF EXISTS coach_plans;
DROP TABLE IF EXISTS baselines;
DROP TABLE IF EXISTS user_goals;

-- +goose StatementEnd
