-- name: InsertGroup :one
INSERT INTO groups (id, name, description, owner_id, deleted, created_at, updated_at)
VALUES ($1, $2, $3, $4, false, $5, $6)
RETURNING *;

-- name: GetGroupRow :one
SELECT * FROM groups
WHERE id = $1 AND NOT deleted
  AND EXISTS (SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2);

-- name: GroupExists :one
SELECT EXISTS (
    SELECT 1 FROM groups g
    WHERE g.id = $1 AND NOT g.deleted
      AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = g.id AND m.user_id = $2)
);

-- name: GroupContainsTitle :one
SELECT EXISTS (
    SELECT 1 FROM groups g
    WHERE g.id = $1 AND NOT g.deleted
      AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = g.id AND m.user_id = $3)
      AND EXISTS (SELECT 1 FROM group_titles t WHERE t.group_id = g.id AND t.title_id = $2)
);

-- name: GroupExistsNotDeleted :one
SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1 AND NOT deleted);

-- name: GroupHasMember :one
SELECT EXISTS (SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2);

-- name: UpdateGroupInfoRow :execrows
UPDATE groups
SET name = $2, description = $3, updated_at = now()
WHERE id = $1 AND NOT deleted;

-- name: SoftDeleteGroupRow :execrows
UPDATE groups
SET deleted = true, deleted_at = now(), updated_at = now()
WHERE id = $1 AND NOT deleted;

-- name: TouchGroup :exec
UPDATE groups SET updated_at = now() WHERE id = $1;

-- name: GetGroupMemberIds :many
SELECT user_id FROM group_members WHERE group_id = $1 ORDER BY user_id;

-- name: GetGroupMemberUsers :many
SELECT u.* FROM group_members m
JOIN users u ON u.id = m.user_id
WHERE m.group_id = $1
ORDER BY u.id;

-- name: UpsertGroupTitle :one
INSERT INTO group_titles (group_id, title_id, title_type, watched, watched_at, added_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (group_id, title_id) DO UPDATE
SET title_type = EXCLUDED.title_type,
    watched = EXCLUDED.watched,
    watched_at = EXCLUDED.watched_at,
    added_at = EXCLUDED.added_at,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetGroupTitleRow :one
SELECT * FROM group_titles WHERE group_id = $1 AND title_id = $2;

-- name: GetGroupTitleRows :many
SELECT * FROM group_titles WHERE group_id = $1 ORDER BY title_id;

-- name: UpdateGroupTitleWatchedRow :one
UPDATE group_titles
SET watched = $3, watched_at = $4, updated_at = $5
WHERE group_id = $1 AND title_id = $2
RETURNING *;

-- name: DeleteGroupTitle :execrows
DELETE FROM group_titles WHERE group_id = $1 AND title_id = $2;

-- name: UpsertGroupTitleSeason :one
INSERT INTO group_title_seasons (group_id, title_id, season, watched, watched_at, added_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (group_id, title_id, season) DO UPDATE
SET watched = EXCLUDED.watched,
    watched_at = EXCLUDED.watched_at,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetGroupTitleSeasonRow :one
SELECT * FROM group_title_seasons WHERE group_id = $1 AND title_id = $2 AND season = $3;

-- name: GetGroupTitleSeasonRows :many
SELECT * FROM group_title_seasons WHERE group_id = $1 ORDER BY title_id, season;

-- name: GetGroupTitleSeasonRowsForTitle :many
SELECT * FROM group_title_seasons WHERE group_id = $1 AND title_id = $2 ORDER BY season;

-- name: InsertGroupFull :exec
-- Migration-only (cmd/mongo-to-postgres): unlike InsertGroup, preserves the
-- original deleted/deleted_at values from the Mongo document.
INSERT INTO groups (id, name, description, owner_id, deleted, deleted_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetGroupRowAnyById :one
-- Migration-only verification read: fetches a group row by id regardless of
-- deleted state or membership (the store's readers filter both out).
SELECT * FROM groups WHERE id = $1;
