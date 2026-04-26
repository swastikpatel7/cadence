-- +goose Up
-- +goose StatementBegin

-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- Shared trigger function
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- users
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

-- users_profile
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

-- connected_accounts
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
    last_error        text
);
CREATE UNIQUE INDEX idx_connected_accounts_provider_user
    ON connected_accounts (provider, provider_user_id);
CREATE UNIQUE INDEX idx_connected_accounts_user_provider
    ON connected_accounts (user_id, provider);
CREATE INDEX idx_connected_accounts_expiry
    ON connected_accounts (expires_at)
    WHERE last_error IS NULL;

-- activities
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
CREATE UNIQUE INDEX idx_activities_source_source_id
    ON activities (source, source_id);
CREATE INDEX idx_activities_user_active
    ON activities (user_id, start_time DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_activities_user_sport
    ON activities (user_id, sport_type, start_time DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_activities_user_recent_covering
    ON activities (user_id, start_time DESC)
    INCLUDE (sport_type, duration_seconds, distance_meters)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_activities_updated_at
    BEFORE UPDATE ON activities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- coach_conversations
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

-- coach_messages
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

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS coach_messages;
DROP TABLE IF EXISTS coach_conversations;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS connected_accounts;
DROP TABLE IF EXISTS users_profile;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
-- Extensions intentionally not dropped: they're shared infrastructure.

-- +goose StatementEnd
