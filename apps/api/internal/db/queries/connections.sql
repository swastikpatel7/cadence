-- name: UpsertConnectedAccount :one
-- Used by the Strava OAuth callback. Idempotent on (user_id, provider)
-- so re-connecting the same user to a Strava account just refreshes the
-- token blob and clears any prior error.
INSERT INTO connected_accounts (
    user_id, provider, provider_user_id, access_token_enc, refresh_token_enc,
    expires_at, scopes, raw_athlete
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id, provider) DO UPDATE
SET provider_user_id  = EXCLUDED.provider_user_id,
    access_token_enc  = EXCLUDED.access_token_enc,
    refresh_token_enc = EXCLUDED.refresh_token_enc,
    expires_at        = EXCLUDED.expires_at,
    scopes            = EXCLUDED.scopes,
    raw_athlete       = EXCLUDED.raw_athlete,
    last_error        = NULL,
    sync_started_at   = NULL,
    sync_progress     = NULL
RETURNING *;

-- name: GetConnectedAccount :one
SELECT * FROM connected_accounts WHERE user_id = $1 AND provider = $2;

-- name: UpdateConnectedAccountTokens :exec
-- Called by the rotating-token TokenSource: when oauth2 refreshes mid-sync,
-- we re-encrypt the new pair and persist. ID-keyed because the worker
-- already has the row in hand.
UPDATE connected_accounts
SET access_token_enc  = $2,
    refresh_token_enc = $3,
    expires_at        = $4
WHERE id = $1;

-- name: DisconnectAccount :exec
-- Soft disconnect: flags the row as user-disconnected. Tokens stay
-- encrypted so accidental re-connect overlap doesn't lose state. The
-- /v1/me/sync POST checks last_error and refuses to enqueue if set.
UPDATE connected_accounts
SET last_error      = 'disconnected by user',
    sync_started_at = NULL,
    sync_progress   = NULL
WHERE user_id = $1 AND provider = $2;

-- name: SetSyncStarted :exec
-- Locks the connection into "syncing" state. POST /v1/me/sync returns
-- 409 if sync_started_at IS NOT NULL.
UPDATE connected_accounts
SET sync_started_at = now(),
    sync_progress   = $3,
    last_error      = NULL
WHERE user_id = $1 AND provider = $2;

-- name: ClearSyncStartedSuccess :exec
-- Worker calls this when the sync loop finishes cleanly.
UPDATE connected_accounts
SET sync_started_at = NULL,
    sync_progress   = NULL,
    last_sync_at    = now(),
    last_error      = NULL
WHERE id = $1;

-- name: ClearSyncStartedFailure :exec
-- Worker calls this on terminal failure (e.g. 401 after refresh attempt).
UPDATE connected_accounts
SET sync_started_at = NULL,
    sync_progress   = NULL,
    last_error      = $2
WHERE id = $1;

-- name: SetSyncProgress :exec
UPDATE connected_accounts
SET sync_progress = $2
WHERE id = $1;
