-- +goose Up
-- Records which member marked a title watched.
--
-- Named for the ACTION, not the viewing: watched is a fact about the group, not
-- a claim that one person saw it. This column answers "who flipped this", which
-- is a different and smaller question than "who watched it".
ALTER TABLE group_titles ADD COLUMN watched_marked_by TEXT REFERENCES users(id) ON DELETE SET NULL;

-- Only the LAST marking, matching the single watched_at the row already keeps.
-- The activity feed holds the full history; this is the current state's author.
--
-- DISTINCT ON takes the most recent event per (group, title), so a title
-- toggled several times gets whoever set it last rather than whoever set it
-- first. Only events that turned it ON are considered: being marked unwatched
-- clears watched_at, so crediting that person would attach a name to a state
-- that no longer exists.
UPDATE group_titles gt
SET watched_marked_by = latest.actor_id
FROM (
    SELECT DISTINCT ON (group_id, title_id)
           group_id, title_id, actor_id
    FROM activity_events
    WHERE kind = 'title_watched_changed'
      AND payload ->> 'watched' = 'true'
    ORDER BY group_id, title_id, created_at DESC
) latest
WHERE latest.group_id = gt.group_id
  AND latest.title_id = gt.title_id
  AND gt.watched IS TRUE;

CREATE INDEX idx_group_titles_watched_marked_by ON group_titles(watched_marked_by);

-- +goose Down
DROP INDEX IF EXISTS idx_group_titles_watched_marked_by;
ALTER TABLE group_titles DROP COLUMN watched_marked_by;
