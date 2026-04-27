-- name: UpsertActivity :one
-- Idempotent on (source, source_id) so re-syncing the same range is safe.
-- We project a handful of summary fields into named columns and dump the
-- full DetailedActivity JSON into `raw` so future code can derive other
-- fields without re-fetching.
INSERT INTO activities (
    user_id, source, source_id, sport_type, name, start_time,
    duration_seconds, distance_meters, elevation_gain_m,
    avg_heart_rate, max_heart_rate, calories, raw
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (source, source_id) DO UPDATE
SET sport_type       = EXCLUDED.sport_type,
    name             = EXCLUDED.name,
    start_time       = EXCLUDED.start_time,
    duration_seconds = EXCLUDED.duration_seconds,
    distance_meters  = EXCLUDED.distance_meters,
    elevation_gain_m = EXCLUDED.elevation_gain_m,
    avg_heart_rate   = EXCLUDED.avg_heart_rate,
    max_heart_rate   = EXCLUDED.max_heart_rate,
    calories         = EXCLUDED.calories,
    raw              = EXCLUDED.raw,
    synced_at        = now(),
    deleted_at       = NULL
RETURNING *;

-- name: UpsertActivityStreams :exec
INSERT INTO activity_streams (activity_id, streams)
VALUES ($1, $2)
ON CONFLICT (activity_id) DO UPDATE
SET streams    = EXCLUDED.streams,
    fetched_at = now();

-- name: ListRecentActivitiesByUser :many
-- Settings page "last N" preview.
SELECT id, name, sport_type, start_time, distance_meters
FROM activities
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY start_time DESC
LIMIT $2;

-- name: CountActivitiesByUser :one
SELECT count(*)::bigint AS total
FROM activities
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: BreakdownActivitiesBySport :many
SELECT sport_type, count(*)::bigint AS total
FROM activities
WHERE user_id = $1 AND deleted_at IS NULL
GROUP BY sport_type
ORDER BY total DESC;

-- name: ListActivitiesInWindow :many
-- Heatmap join: every running activity in [window_start, window_end].
-- Sport-type filter is loose (Run + Trail Run) for the v1 running-only
-- product (insights.md §16 pins non-running out of scope).
SELECT id, source, source_id, sport_type, start_time,
       distance_meters, duration_seconds, avg_heart_rate
FROM activities
WHERE user_id = $1
  AND deleted_at IS NULL
  AND sport_type IN ('Run', 'TrailRun')
  AND start_time >= $2
  AND start_time <  $3
ORDER BY start_time ASC;

-- name: GetActivityForDate :one
-- Used by the session-detail endpoint to find the matched activity for
-- a calendar date. Picks the longest run for that day if multiple exist.
SELECT id, source, source_id, sport_type, start_time,
       distance_meters, duration_seconds, avg_heart_rate
FROM activities
WHERE user_id = $1
  AND deleted_at IS NULL
  AND sport_type IN ('Run', 'TrailRun')
  AND start_time >= $2
  AND start_time <  ($2::date + 1)
ORDER BY distance_meters DESC NULLS LAST
LIMIT 1;
