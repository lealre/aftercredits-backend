-- name: InsertTitle :exec
INSERT INTO titles (
    id, primary_title, type, start_year, rating_aggregate, vote_count,
    added_at, updated_at, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: GetTitleById :one
SELECT * FROM titles WHERE id = $1;

-- name: TitleExists :one
SELECT EXISTS(SELECT 1 FROM titles WHERE id = $1);

-- name: DeleteTitle :execrows
DELETE FROM titles WHERE id = $1;

-- name: ListTitleIds :many
SELECT id FROM titles ORDER BY id;

-- name: UpdateTitle :execrows
UPDATE titles
SET primary_title = $2, type = $3, start_year = $4, rating_aggregate = $5,
    vote_count = $6, added_at = $7, updated_at = $8, metadata = $9
WHERE id = $1;

-- name: CountTitles :one
SELECT count(*) FROM titles;

-- name: GetTitlesPage :many
-- Static total ORDER BY (CONVENTIONS §6 — ends in id ASC). order_by arrives
-- pre-normalized (unknown keys -> ''). No NULLS FIRST/LAST anywhere: Postgres'
-- defaults keep deciding where NULL sort values land (added_at/updated_at are
-- the only nullable keys), exactly as the hand-written version behaved.
SELECT id, primary_title, type, start_year, rating_aggregate, vote_count, added_at, updated_at, metadata
FROM titles
ORDER BY
    CASE WHEN sqlc.arg('order_by')::text IN ('', 'primaryTitle') AND NOT sqlc.arg('descending')::bool THEN primary_title END ASC,
    CASE WHEN sqlc.arg('order_by')::text IN ('', 'primaryTitle') AND sqlc.arg('descending')::bool     THEN primary_title END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'imdbRating' AND NOT sqlc.arg('descending')::bool THEN rating_aggregate END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'imdbRating' AND sqlc.arg('descending')::bool     THEN rating_aggregate END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'startYear'  AND NOT sqlc.arg('descending')::bool THEN start_year END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'startYear'  AND sqlc.arg('descending')::bool     THEN start_year END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'type'       AND NOT sqlc.arg('descending')::bool THEN type END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'type'       AND sqlc.arg('descending')::bool     THEN type END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'voteCount'  AND NOT sqlc.arg('descending')::bool THEN vote_count END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'voteCount'  AND sqlc.arg('descending')::bool     THEN vote_count END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'addedAt'    AND NOT sqlc.arg('descending')::bool THEN added_at END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'addedAt'    AND sqlc.arg('descending')::bool     THEN added_at END DESC,
    CASE WHEN sqlc.arg('order_by')::text = 'updatedAt'  AND NOT sqlc.arg('descending')::bool THEN updated_at END ASC,
    CASE WHEN sqlc.arg('order_by')::text = 'updatedAt'  AND sqlc.arg('descending')::bool     THEN updated_at END DESC,
    id ASC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');
