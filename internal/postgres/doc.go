// Package postgres implements the storage-neutral store.Store interface
// (internal/store) against PostgreSQL.
//
// Migrations live in sql/schema and are applied with goose
// (github.com/pressly/goose/v3). Queries live in sql/queries and are
// compiled by sqlc into internal/database (do not hand-edit that package —
// regenerate it with `sqlc generate`). This package wraps a *pgxpool.Pool
// and the generated database.Queries, mapping rows to/from internal/models
// the same way the Mongo store does.
//
// Tests in this package run against a real postgres:16 container started
// via testcontainers-go (see testharness_test.go), not a mock.
package postgres
