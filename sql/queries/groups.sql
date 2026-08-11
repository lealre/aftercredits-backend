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
INSERT INTO group_titles (group_id, title_id, watched, watched_at, added_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (group_id, title_id) DO UPDATE
SET watched = EXCLUDED.watched,
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

-- name: GetGroupRowAnyById :one
-- Test-only read: fetches a group row by id regardless of deleted state or
-- membership (the store's readers filter both out).
SELECT * FROM groups WHERE id = $1;

-- name: GetGroupTitlesPage :many
-- One-round-trip page of a group's titles: join, NULL-defaulted filters,
-- static total ORDER BY (CONVENTIONS §6 — every branch ends in t.id ASC),
-- and the post-filter total via a window function. order_by arrives
-- pre-normalized (the store method maps unknown keys to ''); watched_at is
-- NULLS LAST in both directions to match the Go comparator this replaces.
-- Title-side keys keep Postgres' default NULL placement (see GetTitlesPage).
--
-- page_size/page_offset are cast to bigint so sqlc generates int64 params —
-- see the same note on GetTitlesPage.
SELECT
    t.id, t.primary_title, t.type, t.start_year, t.rating_aggregate,
    t.vote_count, t.added_at, t.updated_at, t.metadata,
    gt.watched AS gt_watched, gt.watched_at AS gt_watched_at,
    gt.added_at AS gt_added_at, gt.updated_at AS gt_updated_at,
    count(*) OVER () AS total_count
FROM group_titles gt
JOIN titles t ON t.id = gt.title_id
WHERE gt.group_id = sqlc.arg('group_id')
  AND (sqlc.narg('watched')::boolean IS NULL OR gt.watched = sqlc.narg('watched'))
  AND (sqlc.narg('title_types')::text[] IS NULL OR t.type = ANY(sqlc.narg('title_types')::text[]))
ORDER BY
    CASE WHEN sqlc.arg('order_by')::text = 'watched'   AND NOT sqlc.arg('descending')::bool THEN gt.watched END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'watched'   AND sqlc.arg('descending')::bool     THEN gt.watched END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'watchedAt' AND NOT sqlc.arg('descending')::bool THEN gt.watched_at END ASC NULLS LAST,
    CASE WHEN sqlc.arg('order_by')::text = 'watchedAt' AND sqlc.arg('descending')::bool     THEN gt.watched_at END DESC NULLS LAST,
    CASE WHEN sqlc.arg('order_by')::text = 'addedAt'   AND NOT sqlc.arg('descending')::bool THEN gt.added_at END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'addedAt'   AND sqlc.arg('descending')::bool     THEN gt.added_at END DESC,
    CASE WHEN sqlc.arg('order_by')::text IN ('', 'primaryTitle') AND NOT sqlc.arg('descending')::bool THEN t.primary_title END ASC,
    CASE WHEN sqlc.arg('order_by')::text IN ('', 'primaryTitle') AND sqlc.arg('descending')::bool     THEN t.primary_title END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'imdbRating' AND NOT sqlc.arg('descending')::bool THEN t.rating_aggregate END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'imdbRating' AND sqlc.arg('descending')::bool     THEN t.rating_aggregate END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'startYear'  AND NOT sqlc.arg('descending')::bool THEN t.start_year END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'startYear'  AND sqlc.arg('descending')::bool     THEN t.start_year END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'type'       AND NOT sqlc.arg('descending')::bool THEN t.type END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'type'       AND sqlc.arg('descending')::bool     THEN t.type END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'voteCount'  AND NOT sqlc.arg('descending')::bool THEN t.vote_count END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'voteCount'  AND sqlc.arg('descending')::bool     THEN t.vote_count END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'updatedAt'  AND NOT sqlc.arg('descending')::bool THEN t.updated_at END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'updatedAt'  AND sqlc.arg('descending')::bool     THEN t.updated_at END DESC,
    t.id ASC
LIMIT sqlc.arg('page_size')::bigint OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountGroupTitles :one
-- Companion to GetGroupTitlesPage: the window-function total disappears when
-- an out-of-range page returns zero rows, so the store method falls back to
-- this (same WHERE) only in that case. Hot path stays one round trip.
SELECT count(*) FROM group_titles gt
JOIN titles t ON t.id = gt.title_id
WHERE gt.group_id = sqlc.arg('group_id')
  AND (sqlc.narg('watched')::boolean IS NULL OR gt.watched = sqlc.narg('watched'))
  AND (sqlc.narg('title_types')::text[] IS NULL OR t.type = ANY(sqlc.narg('title_types')::text[]));

-- name: GroupHasTitleEntries :one
-- Does the group hold any title entry matching the filters, counting entries
-- whose title is no longer in the catalogue?
--
-- group_titles.title_id carries no FK to titles, so an entry can outlive the
-- title it points at. GetGroupTitlesPage's INNER JOIN hides those orphans,
-- which makes "the group is empty" and "every one of the group's titles was
-- deleted from the catalogue" indistinguishable from its total alone — and
-- the two produce different empty-page envelopes (see GetTitlesFromGroup).
-- The LEFT JOIN is what keeps orphans visible here; it also reproduces the
-- pre-refactor titleType behavior exactly, because an orphan's t.type is NULL
-- and so never matches an explicit type filter.
--
-- EXISTS, not count: the caller only needs zero vs. non-zero, and this runs
-- only on the already-empty path.
SELECT EXISTS (
    SELECT 1 FROM group_titles gt
    LEFT JOIN titles t ON t.id = gt.title_id
    WHERE gt.group_id = sqlc.arg('group_id')
      AND (sqlc.narg('watched')::boolean IS NULL OR gt.watched = sqlc.narg('watched'))
      AND (sqlc.narg('title_types')::text[] IS NULL OR t.type = ANY(sqlc.narg('title_types')::text[]))
);

-- name: GetGroupTitleSeasonRowsForTitles :many
SELECT * FROM group_title_seasons
WHERE group_id = sqlc.arg('group_id') AND title_id = ANY(sqlc.arg('title_ids')::text[])
ORDER BY title_id, season;
