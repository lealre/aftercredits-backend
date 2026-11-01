-- +goose Up
-- When a title was marked watched, as opposed to watched_at, the date the group
-- says it saw the film. watched_at is picked by a member and is often in the
-- past; this is the system time of the marking itself.
--
-- IF NOT EXISTS so this is safe on a database that already has the column
-- from a pre-release build of 012.
ALTER TABLE group_titles ADD COLUMN IF NOT EXISTS watched_marked_at TIMESTAMPTZ;

-- Recover the time of the most recent marking from the activity log. Only
-- events that turned watched ON count, and only rows currently watched are
-- touched, so no timestamp is attached to a state that no longer holds.
UPDATE group_titles gt
SET watched_marked_at = latest.created_at
FROM (
    SELECT DISTINCT ON (group_id, title_id)
           group_id, title_id, created_at
    FROM activity_events
    WHERE kind = 'title_watched_changed'
      AND payload ->> 'watched' = 'true'
    ORDER BY group_id, title_id, created_at DESC
) latest
WHERE latest.group_id = gt.group_id
  AND latest.title_id = gt.title_id
  AND gt.watched IS TRUE
  AND gt.watched_marked_at IS NULL;

-- +goose Down
ALTER TABLE group_titles DROP COLUMN IF EXISTS watched_marked_at;
