-- name: UpsertUserByClerkID :one
-- Idempotent upsert keyed on clerk_user_id. Used by the Clerk webhook
-- handler to keep our users table in sync with Clerk's source of truth.
INSERT INTO users (clerk_user_id, email)
VALUES ($1, $2)
ON CONFLICT (clerk_user_id) DO UPDATE
SET email      = EXCLUDED.email,
    updated_at = now()
RETURNING *;

-- name: GetUserByClerkID :one
SELECT * FROM users WHERE clerk_user_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: DeleteUserByClerkID :exec
-- Hard-deletes a user. Cascade clears users_profile, connected_accounts,
-- activities, and coach_conversations via FK ON DELETE CASCADE.
-- Triggered by Clerk's user.deleted webhook.
DELETE FROM users WHERE clerk_user_id = $1;
