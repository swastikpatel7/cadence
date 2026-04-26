-- +goose Up
-- +goose StatementBegin

-- Streams kept out of activities so list / feed queries don't drag a
-- multi-MB jsonb blob through the planner. Streams are loaded only when
-- something actually wants them (charts, route playback).
CREATE TABLE activity_streams (
    activity_id uuid        PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    streams     jsonb       NOT NULL,
    fetched_at  timestamptz NOT NULL DEFAULT now()
);

-- Per-user manual-sync state on the existing connection row.
--   sync_started_at IS NULL  ⇒ idle. Set on enqueue, cleared on
--                                terminal job completion.
--   sync_progress           ⇒ { processed, total_known, after_ts }
--                                used by the worker to resume after a
--                                JobSnooze on 429.
--   raw_athlete             ⇒ full athlete payload from token-exchange,
--                                used to display the connected athlete
--                                name on the Settings page.
ALTER TABLE connected_accounts
    ADD COLUMN sync_started_at timestamptz,
    ADD COLUMN sync_progress   jsonb,
    ADD COLUMN raw_athlete     jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE connected_accounts DROP COLUMN raw_athlete;
ALTER TABLE connected_accounts DROP COLUMN sync_progress;
ALTER TABLE connected_accounts DROP COLUMN sync_started_at;
DROP TABLE IF EXISTS activity_streams;

-- +goose StatementEnd
