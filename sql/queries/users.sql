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

-- name: AdminExists :one
-- Whether any admin (superuser) account exists at all. Superuser provisioning
-- keys idempotence on this rather than on a fixed username, so re-running the
-- provisioning step can never re-mint a default admin once a real one exists,
-- and deleting a compromised admin lets a fresh one be provisioned from env.
SELECT EXISTS(SELECT 1 FROM users WHERE role = 'admin');

-- name: UserExistsByUsernameOrEmail :one
-- OR-based existence check used to reject a duplicate registration BEFORE the
-- expensive bcrypt hash, rather than hashing first and catching the unique
-- violation afterwards. The unique indexes remain the race backstop.
SELECT EXISTS(
    SELECT 1 FROM users
    WHERE (username <> '' AND username = $1) OR (email <> '' AND email = $2)
);

-- name: DeleteUserById :execrows
-- Soft delete: the row is kept so its authored ratings, comments and owned
-- groups keep a resolvable author instead of orphaning (there are no FKs from
-- those tables to users). Deactivating also revokes access — AuthMiddleware and
-- the SSE open path both refuse an inactive user. Returns the affected row
-- count so a delete of an unknown id is reported as 404, not a silent success.
UPDATE users SET is_active = false, updated_at = now() WHERE id = $1;

-- name: UpdateUserPassword :execrows
-- Rewrites the hash and bumps token_version in one statement, so changing a
-- password immediately invalidates every token minted before it.
UPDATE users
SET password_hash = $2, token_version = token_version + 1, updated_at = now()
WHERE id = $1;

-- name: IncrementUserTokenVersion :execrows
-- "Log out everywhere": invalidate every outstanding token without touching the
-- password.
UPDATE users
SET token_version = token_version + 1, updated_at = now()
WHERE id = $1;

-- name: SetUserActive :execrows
-- Admin kill switch / reinstate. Setting false revokes access on the next
-- request (AuthMiddleware checks is_active) and closes any open SSE stream on
-- its next reconnect.
UPDATE users
SET is_active = $2, updated_at = now()
WHERE id = $1;

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
