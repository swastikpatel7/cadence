-- name: UpsertUserProfile :one
INSERT INTO users_profile (user_id, display_name, timezone, units)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE
SET display_name = EXCLUDED.display_name,
    timezone     = EXCLUDED.timezone,
    units        = EXCLUDED.units,
    updated_at   = now()
RETURNING *;

-- name: GetUserProfile :one
SELECT * FROM users_profile WHERE user_id = $1;
