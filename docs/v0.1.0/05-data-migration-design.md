# v0.1.0 (3/4) — One-time Mongo → Postgres data migration (DESIGN)

**Status:** implemented (this PR).
**Repo:** `github.com/lealre/movies-backend`.
**Umbrella:** sub-project 3 of 4 in the Mongo→Postgres migration (v0.1.0). Sub-projects 1 (store interface, PR #17) and 2 (Postgres layer, PR #18) are MERGED. This builds the one-time data migration; cutover is sub-project 4.

## Goal
A one-time, re-runnable, self-verifying CLI that loads an existing Mongo backup (a `mongodump --out` filesystem dump, as produced by `scripts/backup.sh`) into a fresh goose-migrated Postgres, preserving every id, timestamp, and field value exactly.

## Locked decisions (from brainstorming)
1. **Source = offline mongodump directory.** The tool reads the extracted dump's `<collection>.bson` files directly; no live Mongo needed. The dump location is environment-specific — a flag, never hardcoded. The user provides the real dump only for the final rehearsal; tests use synthetic fixture dumps.
2. **Idempotency = refuse unless `--reset`.** If any target table is non-empty the run aborts; `--reset` truncates all tables inside the load transaction and reloads.
3. **Verification = full read-back compare.** Counts for every table, plus every migrated record re-read through the real `postgres.Store` getters and deep-compared against the model derived from the dump.
4. **Soft-deleted groups ARE migrated**, with their original `deleted`/`deletedAt` values.
5. **Load path = hybrid at the sqlc layer.** The store's `AddRating`/`AddComment`/`CreateGroup` mint fresh uuids and stamp `now()`, so they CANNOT be used. All inserts go through sqlc `database.Queries` (which take explicit ids/timestamps) inside one pgx transaction; fidelity-critical param building is reused from the postgres package (see Components).
6. **Membership**: `group_members` is built from `group.users[]` (the group side wins). Mongo's duplicated `user.groups[]` side is compared as a set during verification; drift is a **warning**, not a failure (deleted groups naturally drop out of the derived list — same behavior the app has today).

## Non-goals
No app wiring/cutover (sub-project 4); no changes to `internal/store` or `internal/models` (frozen); no behavior change to either store implementation; Mongo test suite untouched; no schema (table) changes — the only schema-layer additions are migration-specific sqlc queries.

## Components

### `internal/pgmigration` (new package — all logic, testable)
- **Dump reader** (`dump.go`): a mongodump `.bson` file is concatenated BSON documents (little-endian int32 length prefix, length includes itself). A stream reader decodes each doc into the existing bson-tagged structs `mongodb.UserDb / TitleDb / RatingDb / CommentDb / GroupDb` — the exact types the app writes with, so field names are ground truth by construction. Collections read: `users`, `titles`, `ratings`, `comments`, `groups` (`titlesG` is dead). A missing collection file = empty collection + a loud warning. Dump path resolution: use `<dir>` if it contains `users.bson`, else `<dir>/<dbname>` (from `MONGO_DB`, default `aftercreditsdb`), else a single subdirectory containing `.bson` files; anything ambiguous is an error.
- **Loader** (`load.go`): one `pgx` transaction, all-or-nothing, FK-safe order **users → titles → groups → ratings → comments**:
  - **Reset guard first**: count rows in all 10 tables; any data + no `--reset` → abort; with `--reset` → `TRUNCATE` all 10 inside the transaction.
  - **users** → `CreateUser` (already takes id + created/updated).
  - **titles** → `InsertTitle` with params built by the postgres package's `titleToRow` logic (exported; writes the JSONB `metadata` from `models.Title`), from `TitleDb → models.Title` via the mongodb mapper (exported).
  - **groups** → new sqlc query **`InsertGroupFull`** (id, name, description, owner_id, deleted, deleted_at, created_at, updated_at — the one loader addition `InsertGroup` can't express); then `AddGroupMember` per `users[]` entry, `UpsertGroupTitle` per titles-map entry, `UpsertGroupTitleSeason` per `seasonsWatched` entry — all with original values.
  - **ratings** → `InsertRating` + `InsertRatingSeason` per `seasonsRatings` entry.
  - **comments** → `InsertComment` + `InsertCommentSeason` per `seasonsComments` entry.
- **Verifier** (`verify.go`) — runs after commit, against persisted data:
  1. **Counts**: `users/titles/ratings/comments/groups` vs dump doc counts; `rating_seasons/comment_seasons/group_members/group_titles/group_title_seasons` vs summed map/slice sizes from the dump (including deleted groups).
  2. **Read-back**: every user via `GetUserById`, title via `GetTitleById`, rating via `GetRatingById(id, userId)`, comment via `GetCommentById(id, userId)` — deep-compared against the dump-derived model, honoring the store read conventions (non-nil empty slices on titles; nil season maps when empty). `User.Groups` is compared as a set against the join-derived value with drift downgraded to a warning (decision 6). Non-deleted groups via `GetGroupById(groupId, someMemberId)` (Users compared as sets); deleted groups (and any group unreadable through the store, e.g. zero members) via row-level reads — a small migration-specific sqlc read query (`GetGroupRowAnyById`) plus the existing child-row queries.
  3. **Time comparison** uses `time.Time.Equal` (UTC-normalized); Mongo datetimes are millisecond-precision UTC and Postgres `TIMESTAMPTZ` (µs) holds them exactly.
  4. Any mismatch = failure listed in the report; process exits non-zero. Warnings (membership drift, missing collection files) don't fail the run but are printed.
- **Report**: human-readable summary — per-entity loaded/verified counts, warnings, failures.

### `cmd/mongo-to-postgres/main.go` (thin CLI)
Styled like `cmd/dev-migrations/*` (godotenv, log, emoji status lines). Flags:
- `-dump <dir>` (required) — extracted dump directory (the runbook says `tar -xzf` first; the tool does not read tarballs).
- `-db <postgres-url>` — if empty, built from `POSTGRES_USER/PASSWORD/HOST/PORT/DB` env with the compose defaults (mirrors `mongodb.getMongoURI`'s pattern).
- `--reset` — truncate-and-reload.
Exit 0 only when load + verification succeed (warnings allowed).

### Small exported additions to existing packages
- **`internal/mongodb`**: exported wrappers over the existing unexported Db→model mappers for the five entities (needed by the loader for titles and by the verifier for expected values). One small file; ground-truth mapping stays in one place. (The package dies in sub-project 4 anyway.)
- **`internal/postgres`**: export `titleToRow` (→ `TitleToRow`) and the pgtype converter helpers the loader needs (`timeToTimestamptz`, `ptrToTimestamptz`, `ptrToText`), so the migration writes rows byte-identical to what the store writes. No behavior changes.
- **`sql/queries/groups.sql`**: add `InsertGroupFull` (:exec) and `GetGroupRowAnyById` (:one); `sqlc generate` refreshes `internal/database`.

## Testing (testcontainers-postgres, same harness pattern as `internal/postgres`)
Tests fabricate mongodump-layout fixture dumps by `bson.Marshal`-ing Db structs into a temp dir — no Mongo container needed.
- **Happy path**: fixture with fully-populated entities — a movie + a TV series title (seasons/episodes), users, a live group (members, movie + series titles, `seasonsWatched`), a soft-deleted group with original `deletedAt`, ratings and comments with and without season maps. Load succeeds; verification reports zero failures; original ids/timestamps persisted exactly.
- **Refuse-on-nonempty**: second run without `--reset` aborts before writing; with `--reset` reloads cleanly.
- **Verifier catches corruption**: mutate a row after load (direct SQL) → verification fails, non-zero result.
- **Membership drift**: fixture where a `user.groups[]` disagrees with `group.users[]` → load succeeds, drift reported as warning, verification passes.
- **Dump reader**: empty `.bson` file, missing collection file (warning), multi-document stream round-trip.

## Acceptance criteria
- `go build ./... && go vet ./...` clean; `gofmt` clean; `go` directive stays `1.24.0`; no new module dependencies (mongo-driver, pgx, testcontainers all already present).
- `sqlc generate` clean after the query additions; `internal/database` regenerated, not hand-edited.
- New `internal/pgmigration` tests pass; existing `internal/postgres` and Mongo suites untouched and green.
- `internal/store` and `internal/models` diffs are empty.
- No wiring changes: the app still builds and runs on Mongo.
- Manual gate before PR merge: rehearsal run against the user-provided real dump completes with verification clean (user supplies the dump at that point).

## Risks / notes
- **BSON stream parsing** must be strict: reject truncated documents/trailing garbage; handle 0-byte files.
- `float32` note/rating round-trips through Postgres `REAL` exactly; compare as `float32`.
- `UserDb.Email` is `omitempty` — docs without email decode to `""`; the partial unique indexes (`WHERE` non-empty) already tolerate that.
- The comment movie-vs-series invariant is copied as-is, not enforced — the migration is a copy, not a validator (verification compares as-is).
- The tool is deleted (or archived) in sub-project 4 along with the Mongo code; keep it self-contained.

## Delivery
Branch `v0.1.0-data-migration` off `main`; PR `v0.1.0 (3/4) — Mongo→Postgres data migration`.
