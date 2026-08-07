# mongo-to-postgres

One-time data migration for v0.1.0 (sub-project 3 of the Mongo → Postgres
migration): loads a `mongodump` backup into a fresh Postgres and verifies it
by reading every record back through the Postgres store.

## Runbook

1. Take (or pick) a backup — `scripts/backup.sh` produces
   `backups/mongo_dump_<timestamp>.tar.gz` — and extract it:

       tar -xzf backups/mongo_dump_<timestamp>.tar.gz -C backups/

2. Start Postgres and apply the schema:

       docker compose up -d postgres
       goose -dir sql/schema postgres \
         "postgres://aftercredits:aftercredits@localhost:5432/aftercredits?sslmode=disable" up

3. Run the migration (add `--reset` to wipe and reload a non-empty target):

       go run ./cmd/mongo-to-postgres -dump backups/mongo_dump_<timestamp>

Flags: `-dump` (required) extracted dump dir — either the database directory
containing the `.bson` files or its parent; `-db` Postgres URL (defaults from
`POSTGRES_*` env vars to the compose service); `--reset` truncate-and-reload.

The load runs in a single transaction (a failure commits nothing) and the
run exits non-zero unless verification passes. Warnings (e.g. a user
document's `groups` array drifting from the groups' own member lists — Mongo
stored membership on both sides) are printed but don't fail the run.
