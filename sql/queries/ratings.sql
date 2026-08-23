-- name: InsertRating :one
INSERT INTO ratings (
    id, title_id, user_id, group_id, note, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateRatingRow :one
-- The membership EXISTS is what makes removal from a group revoke write access
-- to that group's ratings: ownership (user_id) alone let an ex-member keep
-- editing rows they authored and re-injecting events into a group they left.
-- The predicate is duplicated on GetRatingRowById and DeleteRatingRow so every
-- by-id path is guarded even if a caller forgets the service-level check.
UPDATE ratings
SET note = $3, updated_at = $4
WHERE ratings.id = $1 AND ratings.user_id = $2
  AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = ratings.group_id AND m.user_id = $2)
RETURNING *;

-- name: GetRatingRowById :one
SELECT * FROM ratings
WHERE ratings.id = $1 AND ratings.user_id = $2
  AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = ratings.group_id AND m.user_id = $2);

-- name: GetRatingRowByUserTitle :one
SELECT * FROM ratings WHERE user_id = $1 AND title_id = $2 AND group_id = $3;

-- name: GetRatingRowsByTitleId :many
SELECT * FROM ratings WHERE title_id = $1 AND group_id = $2;

-- name: GetRatingRowsByTitleIds :many
SELECT * FROM ratings WHERE title_id = ANY($1::text[]) AND group_id = $2;

-- name: DeleteRatingRow :execrows
DELETE FROM ratings
WHERE ratings.id = $1 AND ratings.user_id = $2
  AND EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = ratings.group_id AND m.user_id = $2);

-- name: GetRatingSeasons :many
SELECT * FROM rating_seasons WHERE rating_id = $1;

-- name: GetRatingSeasonsByRatingIds :many
SELECT * FROM rating_seasons WHERE rating_id = ANY($1::text[]);

-- name: DeleteRatingSeasons :exec
DELETE FROM rating_seasons WHERE rating_id = $1;

-- name: InsertRatingSeason :exec
INSERT INTO rating_seasons (
    rating_id, season, rating, added_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5
);
