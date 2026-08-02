-- Placeholder query. sqlc (all versions, incl. v1.30.0) errors with
-- "no queries contained in paths" when sql/queries has zero queries, so a
-- schema-only config cannot generate models.go/db.go on its own. This file
-- exists solely to satisfy that requirement until Task 2 adds real queries
-- (sql/queries/users.sql etc.) — delete this file once real queries exist.

-- name: Placeholder :one
SELECT 1::int AS placeholder;
