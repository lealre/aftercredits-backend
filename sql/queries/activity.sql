-- name: InsertActivityEventRow :one
INSERT INTO activity_events (
    id, group_id, actor_id, actor_name, kind, title_id, title_name, payload, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetActivityFeedRows :many
-- The reader's own actions are excluded: they are recorded so everyone else is
-- notified, not so the actor is notified about themselves. Membership is joined
-- at read time, so leaving a group hides its history immediately.
SELECT e.*, g.name AS group_name
FROM activity_events e
JOIN group_members m ON m.group_id = e.group_id AND m.user_id = sqlc.arg('user_id')
JOIN groups g ON g.id = e.group_id AND NOT g.deleted
WHERE e.actor_id <> sqlc.arg('user_id')
  AND (sqlc.narg('before')::bigint IS NULL OR e.seq < sqlc.narg('before')::bigint)
ORDER BY e.seq DESC
LIMIT sqlc.arg('row_limit')::bigint;

-- name: CountActivityUnread :one
SELECT count(*)
FROM activity_events e
JOIN group_members m ON m.group_id = e.group_id AND m.user_id = sqlc.arg('user_id')
JOIN groups g ON g.id = e.group_id AND NOT g.deleted
LEFT JOIN activity_reads r ON r.user_id = sqlc.arg('user_id')
WHERE e.actor_id <> sqlc.arg('user_id')
  AND e.seq > COALESCE(r.read_seq, 0);

-- name: UpsertActivityRead :exec
-- GREATEST keeps the watermark monotonic: a late request carrying an older seq
-- must not un-read events the user has already seen.
INSERT INTO activity_reads (user_id, read_at, read_seq)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET read_at = EXCLUDED.read_at,
    read_seq = GREATEST(activity_reads.read_seq, EXCLUDED.read_seq);
