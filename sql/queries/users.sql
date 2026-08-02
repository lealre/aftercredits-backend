-- name: CreateUser :exec
INSERT INTO users (
    id, name, email, username, password_hash, avatar_url, role,
    is_active, last_login_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: GetUserById :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsernameOrEmail :one
SELECT * FROM users
WHERE (username = $1 OR $1 = '') AND (email = $2 OR $2 = '');

-- name: GetAllUsers :many
SELECT * FROM users ORDER BY id;

-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);

-- name: DeleteUserById :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateUserInfo :one
UPDATE users
SET name = $2, email = $3, username = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserLastLoginAt :one
UPDATE users
SET last_login_at = now()
WHERE id = $1
RETURNING *;

-- name: AddGroupMember :exec
INSERT INTO group_members (group_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveGroupMember :exec
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- name: GetUserGroupIds :many
SELECT g.id FROM group_members m
JOIN groups g ON g.id = m.group_id
WHERE m.user_id = $1 AND NOT g.deleted;
