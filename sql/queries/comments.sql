-- name: InsertComment :one
INSERT INTO comments (
    id, title_id, user_id, comment, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: UpdateCommentRow :one
UPDATE comments
SET comment = $3, updated_at = $4
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: GetCommentRowById :one
SELECT * FROM comments WHERE id = $1 AND user_id = $2;

-- name: GetUserCommentRowByTitle :one
SELECT * FROM comments WHERE title_id = $1 AND user_id = $2;

-- name: GetCommentRowsByTitleId :many
SELECT * FROM comments WHERE title_id = $1 AND user_id = ANY($2::text[]);

-- name: DeleteCommentRow :execrows
DELETE FROM comments WHERE id = $1 AND user_id = $2;

-- name: GetCommentSeasons :many
SELECT * FROM comment_seasons WHERE comment_id = $1;

-- name: GetCommentSeasonsByCommentIds :many
SELECT * FROM comment_seasons WHERE comment_id = ANY($1::text[]);

-- name: DeleteCommentSeasons :exec
DELETE FROM comment_seasons WHERE comment_id = $1;

-- name: InsertCommentSeason :exec
INSERT INTO comment_seasons (
    comment_id, season, comment, added_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5
);
