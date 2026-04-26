-- =====================================================================
-- Cadence — current database schema (v1)
--
-- This file is the cumulative, post-migration state of the database.
-- It is the canonical "what does the DB look like right now" reference,
-- maintained alongside every migration in ./migrations/.
--
-- Source of truth for changes: ./migrations/*.sql (managed by goose).
-- This file is hand-maintained: every migration must update this file in
-- the SAME PR so reviewers can see the resulting structure.
--
-- Optional cross-check: `pg_dump --schema-only --no-owner --no-privileges`
-- against a freshly migrated dev DB should diff cleanly against this file
-- (modulo cosmetic ordering).
--
-- Section markers `[m: NNNN-name]` show which migration introduced or
-- last modified each piece, so it's easy to track "when did we add X?"
-- =====================================================================


-- ─── Extensions ──────────────────────────────────────────────────────
-- [m: 0001-init]
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid(), digest()
CREATE EXTENSION IF NOT EXISTS vector;     -- reserved for v2 RAG; no columns use it yet


-- ─── Shared trigger function ─────────────────────────────────────────
-- [m: 0001-init]
-- Sets updated_at to now() on UPDATE. Attached to every table that has
-- an updated_at column.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- ─── users ───────────────────────────────────────────────────────────
-- [m: 0001-init]
-- Internal user record, keyed by surrogate UUID.
-- clerk_user_id is the external identity (mirrored from Clerk via the
-- user.created webhook). Email is mirrored too so we can email users
-- without a Clerk roundtrip.
CREATE TABLE users (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_user_id text        NOT NULL,
    email         text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_clerk_user_id ON users (clerk_user_id);
CREATE UNIQUE INDEX idx_users_email_lower   ON users ((lower(email)));

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ─── users_profile ───────────────────────────────────────────────────
-- [m: 0001-init]
-- 1:1 with users. Separated so users.id can be referenced immediately
-- after the Clerk webhook (profile fields are filled in via onboarding).
-- units is constrained to a small enum-as-CHECK for now; if we add more
-- options it will graduate to a proper enum type.
CREATE TABLE users_profile (
    user_id      uuid        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name text,
    timezone     text        NOT NULL DEFAULT 'UTC',
    units        text        NOT NULL DEFAULT 'metric'
                             CHECK (units IN ('metric', 'imperial')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_profile_updated_at
    BEFORE UPDATE ON users_profile
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ─── connected_accounts ──────────────────────────────────────────────
-- [m: 0001-init, 0002-strava_sync]
-- Provider-agnostic OAuth connections. v1 only stores 'strava'.
-- Surrogate UUID PK so a user can disconnect + reconnect to a different
-- provider account without colliding on (provider, provider_user_id).
-- Tokens are AES-256-GCM encrypted at rest via pkg/crypto; nonces are
-- prefixed inside the bytea blob.
--
-- Manual-sync state (sync_started_at, sync_progress) is stored on the row
-- itself so the worker can resume cleanly after a 429 JobSnooze, and
-- POST /v1/me/sync can refuse with 409 while a sync is in flight.
-- raw_athlete carries the athlete payload from the token-exchange so the
-- Settings page can show the connected athlete's name without an extra
-- Strava round-trip.
CREATE TABLE connected_accounts (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          text        NOT NULL,
    provider_user_id  text        NOT NULL,
    access_token_enc  bytea       NOT NULL,
    refresh_token_enc bytea       NOT NULL,
    expires_at        timestamptz NOT NULL,
    scopes            text[]      NOT NULL,
    connected_at      timestamptz NOT NULL DEFAULT now(),
    last_sync_at      timestamptz,
    last_error        text,
    sync_started_at   timestamptz,                          -- [m: 0002-strava_sync]
    sync_progress     jsonb,                                -- [m: 0002-strava_sync]
    raw_athlete       jsonb                                 -- [m: 0002-strava_sync]
);

-- One active connection per (provider, provider_user_id) globally.
CREATE UNIQUE INDEX idx_connected_accounts_provider_user
    ON connected_accounts (provider, provider_user_id);

-- One active connection per provider per user (prevents two concurrent
-- Strava connections for the same Cadence user).
CREATE UNIQUE INDEX idx_connected_accounts_user_provider
    ON connected_accounts (user_id, provider);

-- Drives the proactive token-refresh cron. Partial index keeps the
-- working set tiny — we ignore connections that have errored out.
CREATE INDEX idx_connected_accounts_expiry
    ON connected_accounts (expires_at)
    WHERE last_error IS NULL;


-- ─── activities ──────────────────────────────────────────────────────
-- [m: 0001-init]
-- Normalized activity rows. `raw` JSONB stores the full provider
-- payload, so we can derive newly-needed fields without backfilling
-- from Strava.
--
-- (source, source_id) UNIQUE makes webhook handling idempotent — Strava
-- can fire the same event twice and we don't duplicate.
--
-- deleted_at supports Strava delete events without losing audit trail
-- (we soft-delete; the row stays for analytics & coach context).
--
-- Partitioning: NOT partitioned in v1 (single user, low volume).
-- See "Optimization watchlist" at the bottom of this file for the
-- monthly-partition-by-start_time trigger threshold.
CREATE TABLE activities (
    id                  uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source              text           NOT NULL,
    source_id           text           NOT NULL,
    sport_type          text           NOT NULL,
    name                text           NOT NULL,
    start_time          timestamptz    NOT NULL,
    duration_seconds    integer        NOT NULL,
    distance_meters     numeric(10, 2),
    elevation_gain_m    numeric(8, 2),
    avg_heart_rate      integer,
    max_heart_rate      integer,
    avg_pace_sec_per_km numeric(8, 2),
    calories            integer,
    raw                 jsonb          NOT NULL,
    synced_at           timestamptz    NOT NULL DEFAULT now(),
    updated_at          timestamptz    NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

-- Idempotent upsert key (matched in `INSERT ... ON CONFLICT (source, source_id)`).
CREATE UNIQUE INDEX idx_activities_source_source_id
    ON activities (source, source_id);

-- Primary read pattern: feed for one user, most recent first.
-- Partial index over not-deleted rows = smaller, hotter cache.
CREATE INDEX idx_activities_user_active
    ON activities (user_id, start_time DESC)
    WHERE deleted_at IS NULL;

-- Sport-filtered feed (e.g. "show me only my runs").
CREATE INDEX idx_activities_user_sport
    ON activities (user_id, sport_type, start_time DESC)
    WHERE deleted_at IS NULL;

-- Covering index for coach context-build queries that scan recent rows
-- and only need a few columns. Avoids the heap fetch.
CREATE INDEX idx_activities_user_recent_covering
    ON activities (user_id, start_time DESC)
    INCLUDE (sport_type, duration_seconds, distance_meters)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_activities_updated_at
    BEFORE UPDATE ON activities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ─── activity_streams ────────────────────────────────────────────────
-- [m: 0002-strava_sync]
-- Sibling to activities, kept separate so list / feed queries on
-- activities don't drag a multi-MB streams blob through the planner.
-- Streams are only loaded when something actually wants them (charts,
-- route playback). Schema is intentionally generic: `streams` is a
-- key_by_type=true response from Strava's /streams endpoint, but a
-- future provider could land a different jsonb shape.
CREATE TABLE activity_streams (
    activity_id uuid        PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    streams     jsonb       NOT NULL,
    fetched_at  timestamptz NOT NULL DEFAULT now()
);


-- ─── coach_conversations ─────────────────────────────────────────────
-- [m: 0001-init]
-- A user's chat threads with the AI coach. updated_at tracks last
-- activity so we can sort the sidebar list by recency.
CREATE TABLE coach_conversations (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_coach_conversations_user_recent
    ON coach_conversations (user_id, updated_at DESC);

CREATE TRIGGER trg_coach_conversations_updated_at
    BEFORE UPDATE ON coach_conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ─── coach_messages ──────────────────────────────────────────────────
-- [m: 0001-init]
-- Individual messages in a conversation. `citations` is a JSONB list of
-- `{type: 'activity'|'metric', id: string}` objects extracted from the
-- assistant response post-stream.
--
-- token_count is recorded for context-window budget tracking.
--
-- No embedding column in v1 — added in v2 with the dimension that
-- matches the embedding model we actually pick (Voyage / Cohere / OpenAI
-- all use different dims).
CREATE TABLE coach_messages (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid        NOT NULL REFERENCES coach_conversations(id) ON DELETE CASCADE,
    role            text        NOT NULL
                                CHECK (role IN ('user', 'assistant', 'system')),
    content         text        NOT NULL,
    citations       jsonb,
    token_count     integer,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_coach_messages_conversation
    ON coach_messages (conversation_id, created_at);


-- ─── River (Postgres job queue) ──────────────────────────────────────
-- River creates and owns its tables (river_job, river_leader,
-- river_queue, river_client, etc.) via its migrate command, run on
-- worker startup. We do NOT duplicate those CREATE TABLEs here — they
-- are River's internal API and version with River releases.
-- Reference: https://riverqueue.com/docs/database-migrations


-- =====================================================================
-- Optimization watchlist
--
-- Re-evaluate at each scaling milestone. Trigger thresholds are rough
-- guides — measure before partitioning.
-- =====================================================================
--
-- [activities partition] When activities crosses ~5M rows OR weekly
-- aggregate queries cross 200ms, partition by RANGE(start_time)
-- monthly. Plan: detach pre-partition rows into activities_legacy,
-- create partitioned activities_new with monthly partitions, swap.
-- Foreign keys to activities will need rework (composite FKs on
-- partitioned tables).
--
-- [coach_messages.citations GIN] If the UI ever filters/searches by
-- cited activity_id, add `CREATE INDEX ... USING GIN (citations)`.
-- Skip until that feature exists.
--
-- [coach_messages.embedding] (v2) Add `embedding vector(<DIM>)` plus
-- HNSW index `... USING hnsw (embedding vector_cosine_ops)`. Pick DIM
-- when the embedding model is selected (1024 Cohere / 1536 OpenAI /
-- 1024 Voyage-3-lite / etc.).
--
-- [weekly_totals materialized view] If coach context-build becomes a
-- hot path (it shouldn't — it's per-message, low QPS), add a refreshable
-- MV keyed on (user_id, iso_week_start). REFRESH CONCURRENTLY after
-- activity ingest.
--
-- [connected_accounts.expires_at] The partial index assumes refresh
-- failures are rare. If we ever see >5% of connections in last_error
-- state, drop the partial predicate.
-- =====================================================================
