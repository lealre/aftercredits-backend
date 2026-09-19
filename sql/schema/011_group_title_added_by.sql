-- +goose Up
-- Records which member put a title into a group.
--
-- The activity feed already logs this as an event, but an event answers "what
-- happened" and disappears from view; this answers "whose is this" for as long
-- as the row exists. group_titles had no author column at all, which is why the
-- feed could not be rebuilt from state.
ALTER TABLE group_titles ADD COLUMN added_by TEXT REFERENCES users(id) ON DELETE SET NULL;

-- SET NULL, not CASCADE: deleting a user must never delete a group's films.
-- The column is nullable for the same reason it is not backfilled to a
-- fallback — "nobody recorded this" is a true answer, and inventing an author
-- would be worse than admitting the gap.

-- Recover what can be recovered. Titles added since the activity feed shipped
-- have an event naming the actor; anything older has no record anywhere and
-- stays NULL.
UPDATE group_titles gt
SET added_by = a.actor_id
FROM activity_events a
WHERE a.kind = 'title_added'
  AND a.group_id = gt.group_id
  AND a.title_id = gt.title_id
  AND gt.added_by IS NULL;

CREATE INDEX idx_group_titles_added_by ON group_titles(added_by);

-- +goose Down
DROP INDEX IF EXISTS idx_group_titles_added_by;
ALTER TABLE group_titles DROP COLUMN added_by;
