-- name: InsertActivityEventRow :one
INSERT INTO activity_events (
    id, group_id, actor_id, actor_name, kind, title_id, title_name, payload, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetActivityFeedRows :many
-- Visibility comes from activity_visible_events (your groups, never your own
-- actions, nothing from a deleted group), so this query and the badge cannot
-- drift apart. Membership is part of that view, so leaving a group hides its
-- history immediately.
--
-- read_by_me is the reader's own state, joined per row: the same event is read
-- for one member and unread for another, which is why it cannot live on
-- activity_events.
SELECT v.id, v.seq, v.group_id, v.actor_id, v.actor_name, v.kind, v.title_id,
       v.title_name, v.payload, v.created_at, v.group_name,
       (v.seq <= COALESCE(f.floor_seq, 0) OR r.event_id IS NOT NULL)::boolean AS read_by_me
FROM activity_visible_events v
LEFT JOIN activity_read_floors f ON f.user_id = v.reader_id
LEFT JOIN activity_event_reads r ON r.event_id = v.id AND r.user_id = v.reader_id
WHERE v.reader_id = sqlc.arg('user_id')
  AND (sqlc.narg('before')::bigint IS NULL OR v.seq < sqlc.narg('before')::bigint)
ORDER BY v.seq DESC
LIMIT sqlc.arg('row_limit')::bigint;

-- name: CountActivityUnread :one
-- The badge is exactly "visible events with no read row for me" — the same
-- view, the same anti-join, as the read_by_me column above.
SELECT count(*)
FROM activity_visible_events v
LEFT JOIN activity_read_floors f ON f.user_id = v.reader_id
LEFT JOIN activity_event_reads r ON r.event_id = v.id AND r.user_id = v.reader_id
WHERE v.reader_id = sqlc.arg('user_id')
  AND v.seq > COALESCE(f.floor_seq, 0)
  AND r.event_id IS NULL;

-- name: MarkActivityEventRead :one
-- Marks one event read for one reader, and answers whether that event was
-- visible to them at all: the INSERT's source is the visibility view, so a
-- non-member (or an unknown id) selects no row and the statement returns
-- nothing, which the store turns into ErrRecordNotFound.
--
-- ON CONFLICT ... DO UPDATE, rather than DO NOTHING, is what makes the two
-- outcomes distinguishable while staying idempotent: DO NOTHING also returns no
-- row for an event already read, which is a success. The update assigns read_at
-- to itself, so a second click keeps the moment of the first read and only ever
-- costs a rewritten row version.
--
-- The CTE separates the two questions the caller conflates. `visible` answers
-- "may this reader see this event at all", which decides 204 vs 404, and is
-- returned regardless of read state. The INSERT runs only for events ABOVE the
-- floor: at or below it the event is already read, so writing an exception row
-- would add a row the floor already covers — the redundancy this whole change
-- exists to remove. Marking such an event read is therefore a no-op that still
-- reports success, which is what idempotent means here.
WITH visible AS (
    SELECT v.id, v.seq, v.reader_id, COALESCE(f.floor_seq, 0) AS floor_seq
    FROM activity_visible_events v
    LEFT JOIN activity_read_floors f ON f.user_id = v.reader_id
    WHERE v.reader_id = sqlc.arg('user_id')
      AND v.id = sqlc.arg('event_id')
), inserted AS (
    INSERT INTO activity_event_reads (user_id, event_id, read_at)
    SELECT reader_id, id, sqlc.arg('read_at')::timestamptz
    FROM visible
    WHERE seq > floor_seq
    ON CONFLICT (user_id, event_id) DO UPDATE
    SET read_at = activity_event_reads.read_at
    RETURNING event_id
)
SELECT id AS event_id FROM visible;

-- name: MarkAllActivityEventsRead :exec
-- Raises the reader's floor to the newest event they can see, then drops the
-- exception rows the floor now covers. One integer written and a bounded
-- delete, instead of an insert per visible event — the whole point of the floor.
--
-- GREATEST keeps the floor monotonic: a request that raced a newer one must not
-- lower it and silently un-read what the other just cleared.
--
-- The DELETE is not just tidiness. Without it the exception rows accumulate
-- exactly as they did before the floor existed, and the storage this change
-- exists to save is never actually saved.
--
-- max(seq) over no visible events is NULL, hence the COALESCE: a reader in no
-- groups gets a floor of 0, which reads as "nothing is below it" rather than
-- failing.
WITH raised AS (
    INSERT INTO activity_read_floors (user_id, floor_seq, read_at)
    SELECT sqlc.arg('user_id'), COALESCE(max(v.seq), 0), sqlc.arg('read_at')::timestamptz
    FROM activity_visible_events v
    WHERE v.reader_id = sqlc.arg('user_id')
    ON CONFLICT (user_id) DO UPDATE
    SET floor_seq = GREATEST(activity_read_floors.floor_seq, EXCLUDED.floor_seq),
        read_at   = EXCLUDED.read_at
    RETURNING floor_seq
)
DELETE FROM activity_event_reads r
USING activity_events e, raised
WHERE r.user_id = sqlc.arg('user_id')
  AND e.id = r.event_id
  AND e.seq <= raised.floor_seq;

-- name: NotifyActivityEvent :exec
-- Fired inside the insert's transaction, so it is delivered only if that commit
-- succeeds. The payload is the event id, not the row: pg_notify caps payloads at
-- 8000 bytes, and the listener reads the row once and fans it out.
SELECT pg_notify('activity_events', sqlc.arg('event_id')::text);

-- name: GetActivityEventById :one
-- Used by the LISTEN loop to turn a notified id into the row it pushes. No
-- visibility predicate here: the loop has no reader, and the hub filters per
-- subscriber.
--
-- read_by_me is FALSE for the same reason: a pushed event was just written, so
-- it is unread for every subscriber it reaches. The column is selected rather
-- than omitted so this row stays field-for-field identical to
-- GetActivityFeedRows' — the store converts directly between the two.
SELECT e.id, e.seq, e.group_id, e.actor_id, e.actor_name, e.kind, e.title_id,
       e.title_name, e.payload, e.created_at, g.name AS group_name,
       FALSE AS read_by_me
FROM activity_events e
JOIN groups g ON g.id = e.group_id
WHERE e.id = $1;
