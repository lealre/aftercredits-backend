// Package pgmigration implements the one-time Mongo -> Postgres data
// migration (v0.1.0 sub-project 3): reading a mongodump filesystem backup
// into the mongodb package's Db structs, loading it into a goose-migrated
// Postgres with original ids and timestamps preserved, and verifying the
// result by reading everything back. Consumed by cmd/mongo-to-postgres.
package pgmigration
