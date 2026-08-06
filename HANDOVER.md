# Handover — v0.1.0 Mongo → Postgres migration

Working handover for continuing the **v0.1.0** effort in a fresh session (possibly a different agent/machine). It captures state, decisions, how to run things, and what's next. Design docs live in [`docs/v0.1.0/`](docs/v0.1.0/).

> This is a public repo. Keep commits, code, and docs free of personal or private-infrastructure references (no deploy hosts/IPs, no secrets, no third-party workspace links).

## What v0.1.0 is
Migrate the backend's datastore from MongoDB to **Postgres**, behavior-preserving (no API/response changes, no new features). The data is mostly relational (users, groups, memberships, ratings, comments) with one document-shaped entity (titles = external metadata). It's decomposed into **4 sequential, independently-shippable sub-projects**, each with its own spec → plan → execution.

| # | Sub-project | Status |
|---|-------------|--------|
| 1 | **Store interface + neutral models** — services depend on `store.Store` + `internal/models`, never on the concrete DB. Still on Mongo. | ✅ merged (PR #17) |
| 2 | **Postgres layer** — goose schema + sqlc queries + `postgres.Store` implementing the interface; testcontainers-postgres tests. Not wired into the app. | ✅ merged (PR #18) |
| 3 | **One-time Mongo → Postgres data migration** — script that reads the Mongo backup, transforms, loads Postgres, verifies. | ⬜ **NEXT** |
| 4 | **Cutover + deploy** — wire the app to `postgres.Store`, run goose in the deploy, drop Mongo/db-setup, switch backups to `pg_dump`, delete Mongo code. | ⬜ pending |

## Locked design decisions (apply to all sub-projects)
- **Target: Postgres** (not SQLite). Driver: **pgx/v5** (`pgxpool`); queries via **sqlc** (v2, engine `postgresql`) → generated `internal/database`; migrations via **goose** (`sql/schema/NNN_*.sql`, `-- +goose Up/Down`).
- **TEXT primary keys** — keep existing string ids (Mongo hex ObjectIds for users/groups/ratings/comments; imdb `tt…` for titles). Makes sub-project 3 a straight copy with FKs aligned.
- **Titles = hybrid**: query columns (id, primary_title, type, start_year, rating_aggregate, vote_count, added_at, updated_at) + a `metadata JSONB` that stores the **complete `models.Title`** (source of truth on read; columns are denormalized query copies).
- **Child tables** for the per-season maps: `rating_seasons`, `comment_seasons`, `group_title_seasons` — written transactionally as whole-map replace.
- **Membership = one join table** `group_members` (Mongo stored it twice, `user.groups[]` + `group.users[]`; Postgres derives both sides from the join).
- **Portability boundary**: `internal/store.Store` (42 methods) + `internal/models` are the fixed contract. Both `*mongodb.DB` and `*postgres.Store` implement it (`var _ store.Store = (*T)(nil)`). Services/api import `store` + `models`, never a concrete DB package.
- **Behavior fidelity to Mongo is the spec.** The Mongo implementation (`internal/mongodb/*`) is ground truth; the Postgres store reproduces its exact semantics, including nil-vs-empty conventions (e.g. title read returns non-nil empty slices for Directors/Writers/Stars/OriginCountries/SpokenLanguages/Interests/Seasons/Episodes; season maps are nil when there are no child rows), error sentinels (`store.ErrRecordNotFound`, `store.ErrDuplicatedRecord`), and the TV-series watched recompute (top-level `watched` = OR of seasons, `watchedAt` = MAX among watched seasons).

## Repository map (relevant to v0.1.0)
- `internal/store/` — the `Store` interface (`store.go`) + sentinel errors (`errors.go`). **Frozen** — do not change.
- `internal/models/` — storage-neutral domain types (no bson/sql tags). **Frozen.**
- `internal/mongodb/` — Mongo implementation (ground truth for behavior).
- `internal/postgres/` — Postgres implementation: `store.go` (type + `New` + interface assertion), `mappers.go` (row↔model + shared helpers `isUniqueViolation`/`notFound`/pgtype converters + `assembleSeasons*`), `users.go`, `titles.go`, `ratings.go`, `comments.go`, `groups.go`, `testharness_test.go` (testcontainers), `*_test.go`.
- `internal/database/` — **sqlc-generated** (do not hand-edit; regenerate with `sqlc generate`).
- `sql/schema/001_init.sql` — goose migration (the 10-table schema).
- `sql/queries/*.sql` — sqlc query sources.
- `sqlc.yaml` — sqlc config (v2, pgx/v5, out `internal/database`).
- `docker-compose.yaml` — `mongo` + `postgres:16` services for local dev.
- `docs/v0.1.0/` — the design docs (see below).

## Design docs (read these first)
- [`docs/v0.1.0/01-umbrella-and-store-interface-design.md`](docs/v0.1.0/01-umbrella-and-store-interface-design.md) — the whole-project umbrella + sub-project 1 design.
- [`docs/v0.1.0/02-store-interface-plan.md`](docs/v0.1.0/02-store-interface-plan.md) — sub-project 1 implementation plan.
- [`docs/v0.1.0/03-postgres-store-design.md`](docs/v0.1.0/03-postgres-store-design.md) — sub-project 2 (Postgres layer) design.
- [`docs/v0.1.0/04-postgres-store-plan.md`](docs/v0.1.0/04-postgres-store-plan.md) — sub-project 2 implementation plan.

## How to run
```bash
# Start Postgres (postgres:16; matches the version the tests use).
# Host 5432 may already be taken — override with POSTGRES_PORT if so.
docker compose up -d postgres
# Defaults: user/password/db all "aftercredits", port 5432, volume aftercredits-postgres-data.

# Apply the schema (goose CLI — the migration file has goose markers, so do NOT psql it directly).
goose -dir sql/schema postgres \
  "postgres://aftercredits:aftercredits@localhost:5432/aftercredits?sslmode=disable" up

# Build / vet / test. The postgres store tests spin up their own postgres:16 testcontainer (Docker required).
go build ./... && go vet ./...
go test ./internal/postgres/ -count=1
```

## Sub-project 3 — data migration (the NEXT task)
**Goal:** a one-time, re-runnable, verified script that loads existing Mongo data into a fresh Postgres. Not yet designed — start with the `superpowers:brainstorming` skill to produce its own spec, then `writing-plans`, then execute.

Starting considerations (to confirm during brainstorming, not decisions yet):
- **Source of truth for the data:** a Mongo backup produced by `mongodump` (filesystem BSON, `--out` dump) rather than a live DB, so the migration is offline and repeatable. The backup's location is environment-specific — parameterize it, do not hardcode.
- **Load path options:** (a) read BSON → build `models.*` → call the existing `postgres.Store` `Add*`/`Create*` methods (reuses all the fidelity logic, transactions, and mappers already written and reviewed in sub-project 2 — recommended); vs (b) bulk `COPY`/inserts for speed. Weigh correctness-reuse vs throughput; dataset is small, so (a) is likely best.
- **Ids:** copy straight (TEXT PKs preserve Mongo hex + imdb ids), so cross-entity references line up without remapping.
- **Ordering / FKs:** load titles + users before ratings/comments/group_titles that reference them; groups before members/titles/seasons.
- **Verification:** per-collection row counts Mongo vs Postgres + spot-check a few fully-populated documents (a TV series title with seasons/episodes; a group with members + watched seasons; ratings/comments with season maps) for field-level equality.
- **Idempotency:** decide whether re-running truncates-and-reloads or upserts.

## Workflow conventions used here
- Process: `superpowers:brainstorming` (design → spec) → `superpowers:writing-plans` (plan) → `superpowers:subagent-driven-development` (fresh implementer per task + per-task review + final whole-branch review). Specs/plans are drafted under `docs/superpowers/` (git-ignored working scratch); curated copies for this handover live in `docs/v0.1.0/`.
- **Public repo:** no co-author trailers; keep personal/private-infrastructure details out of commits, code, and PRs.
- **Backend tests:** `t.Run` subtests + response-returning setup helpers + DB helpers; no inline HTTP in assertions.
- **Versioning:** the app version lives in the backend; the frontend repo is separate and not tagged in lock-step.
- **Gotchas:** editor/LSP "undefined method" or "no packages found" diagnostics after a subagent edit are frequently **stale** — trust `go build`/`go vet`/`go test` at HEAD. Run `sqlc generate` after editing any `sql/queries/*.sql`. The `go` directive is pinned at `1.24.0` — don't let a dependency bump it.

## Deferred minors carried forward (from sub-project 2 reviews — none blocking)
- `GetAllUsers` does N+1 group-id lookups (fine at current scale; batch later).
- `postgres.AddTitle` drops Mongo's empty-id guard (unreachable — the provider always sets the id).
- `google/uuid` was promoted from an indirect to a direct dependency for id generation (no new module; ids are opaque TEXT).
- A weak `UpdatedAt` assertion in one users test.
