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

-- name: GetTitleTypes :many
SELECT id, type FROM titles WHERE id = ANY($1::text[]);

-- name: ListTitleIds :many
SELECT id FROM titles ORDER BY id;

-- name: UpdateTitle :execrows
UPDATE titles
SET primary_title = $2, type = $3, start_year = $4, rating_aggregate = $5,
    vote_count = $6, added_at = $7, updated_at = $8, metadata = $9
WHERE id = $1;
