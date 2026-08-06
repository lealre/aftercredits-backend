# v0.1.0 — Mongo → Postgres migration (DESIGN)

**Status:** overview + Sub-project 1 approved 2026-07-31.
**Repo:** `github.com/lealre/movies-backend`. goose/sqlc conventions follow a conventional goose+sqlc layout.

---

## Umbrella: v0.1.0 (the whole project)

**Motivation.** The data is mostly relational (users, groups, memberships, ratings, comments) with one genuinely document-shaped entity (titles = external TMDB/OMDb metadata). Modeling the relational parts on MongoDB has cost: 0 transactions (multi-doc updates like group-delete/leave are non-atomic), 6 hand-written Go backfill migrations, aggregation pipelines for simple sorts, and a two-sided many-to-many (`user.groups[]` + `group.users[]`) kept in sync by hand. Postgres + a migration tool + FKs + transactions fixes all of that.

**Locked decisions (from brainstorming):**
1. **Target: Postgres** (not SQLite).
2. **Titles storage = hybrid**: real columns for queried/sorted fields (id, primary_title, type, start_year, rating_aggregate, vote_count, added_at, updated_at, deleted flags) + a `metadata JSONB` column for the fat nested stuff (seasons, episodes, cast, images, genres, plot). The relational entities (users, groups, group_members join table, group_titles, group_title_seasons, ratings, comments) are normalized tables.
3. **Portability model = `Store` interface, sqlc behind it.** Services depend on a `Store` interface speaking storage-neutral domain types; the Postgres implementation uses sqlc internally. Swapping DBs later = new `Store` impl, services untouched.
4. **Tooling = goose + sqlc** (conventional goose+sqlc layout): goose migrations `sql/schema/NNN_*.sql` (`-- +goose Up/Down`), sqlc queries `sql/queries/*.sql` (`-- name: X :one|:many|:exec`, `RETURNING *`, `COALESCE` for partial updates), `sqlc.yaml` v2 engine `postgresql` → generated code in `internal/database`, over a `database/sql` `DBTX` with `WithTx`.
5. **Sequencing = portability refactor first**, then Postgres layer, then data migration, then cutover.

**Decomposition (4 sequential sub-projects, each its own spec → plan → execution, each independently shippable):**
1. **Store interface (still on Mongo)** — this document's detailed design below.
2. **Postgres layer** — goose schema + sqlc queries + `postgres.Store` implementing the interface; testcontainers-postgres; not wired into the running app yet.
3. **One-time Mongo → Postgres data migration** — a script reading the mongodump backup (filesystem BSON `--out` dump), transforming, loading Postgres; with verification (row counts + spot checks).
4. **Cutover + deploy** — swap wiring to `postgres.Store`; production deploy adds a Postgres container + runs goose + drops Mongo/db-setup; backups switch to `pg_dump`; delete Mongo code.

**Non-goals for v0.1.0:** no new product features; no API/response-shape changes (behavior-preserving throughout); no auth changes; the `titleprovider` (external API) layer is untouched.

**Surface being migrated (sizing):** 5 entities (users, groups, ratings, comments, titles); **~50 DB methods** on `*mongodb.DB`; referenced at **~52 sites** in `internal/services`.

---

## Sub-project 1 — `Store` interface (still on Mongo)

**Goal.** Services depend on a `Store` interface (storage-neutral) instead of `*mongodb.DB`. Zero behavior change; the full existing integration suite stays green. This is the foundation that makes Sub-project 2 "implement `Store` with Postgres/sqlc."

### The type boundary (the core of this sub-project)
Today: DB methods return storage types (`mongodb.GroupDb`, `mongodb.UserDb`, …); services import those and map them to API responses. For real portability the interface must NOT expose `mongodb.*` types.

Introduce **`internal/models`** — storage-neutral domain types (no `bson`/`sql` tags): `models.User`, `models.Group`, `models.GroupTitle`, `models.Title` (+ its sub-types: Image, Rating, Metacritic, Person, Season, Episode, ReleaseDate, CodeName, Interest), `models.Rating`, `models.Comment`, and the small value types (SeasonsWatched, SeasonRating, SeasonComment). Where an existing service-layer type is already storage-neutral and a natural fit (e.g. `titles.Title` has only json tags), **promote/move it into `models`** rather than defining a third near-identical struct — avoid triple-typing. The `mongodb.*Db` structs remain as the **storage representation only** and gain mapper funcs `*Db ↔ models.*` (mirrors the pattern `internal/titleprovider` already uses with neutral types).

### Packages
- `internal/models` (new) — neutral domain types + their zero-dependency helpers. Imports nothing from `mongodb`/`services`.
- `internal/store` (new) — the `Store` interface (one interface, ~50 methods, grouped by entity in the file with comments; mirrors how sqlc emits a single `Querier`). Consumes/returns `models.*` and stdlib/`context` only. **Also holds the storage-neutral sentinel errors** the interface's contract exposes — `store.ErrRecordNotFound`, `store.ErrDuplicatedRecord` — since services currently branch on `errors.Is(err, mongodb.ErrRecordNotFound)` and must not import `mongodb`. The mongo impl translates its internal errors (`mongo.ErrNoDocuments`, duplicate-key) to these `store` sentinels at the boundary; both impls return the same sentinels so service error-mapping is DB-agnostic.
- `internal/mongodb` (existing) — `*DB` keeps its `*Db` BSON structs internally; each public method's signature changes to accept/return `models.*`, doing `*Db ↔ models.*` mapping at the boundary. `*mongodb.DB` satisfies `store.Store` (compile-time assert `var _ store.Store = (*DB)(nil)`).
- `internal/services/*` — take `store.Store` instead of `*mongodb.DB`; their existing DB→response mappers change source type from `mongodb.*Db` to `models.*`.
- `internal/api` — `API.Db` becomes `store.Store`; `NewAPI(db store.Store, ...)`.
- wiring (`internal/server`) — the only place that constructs the concrete `mongodb.DB` and passes it as a `store.Store`.

### What does NOT change
API request/response shapes and status codes; the `titleprovider` layer; the auth/JWT layer; the config layer; test *behavior* (only the concrete type threaded through `NewServerWithProvider` — which already passes a `*mongo.Client` and builds `mongodb.NewDB`, that stays; the DB it builds now also satisfies `store.Store`).

### Testing
The existing testcontainers-mongo integration suite is the safety net — it must stay 100% green with no test changes beyond any needed type imports. This is a pure, behavior-preserving refactor: same requests, same responses. `go build/vet`, `gofmt`, `go test ./... -count=1` all green.

### Acceptance criteria
- New `internal/models` + `internal/store` packages; `var _ store.Store = (*mongodb.DB)(nil)` compiles.
- `grep -rn "movies-backend/internal/mongodb" internal/services internal/api` → **empty** (services/api import `store` + `models`, never `mongodb`). The sole `mongodb` import outside its package is the wiring entrypoint in `internal/server`.
- No `mongodb.*` type appears in any `store.Store` method signature.
- Behavior-preserving: full unit + integration suites green; `grep -rnE "api\.Db\." internal/api/*.go` still empty (CONVENTIONS.md §1 intact).
- No API/response/JSON changes (a diff of any endpoint's response is byte-identical).

### Risks / notes
- **Effort concentration:** most of the work is the `*Db ↔ models.*` mapping and re-pointing service mappers. It's mechanical and fully test-covered, but touches many files — good candidate for staged, per-entity execution (users → titles → ratings → comments → groups), keeping the suite green after each entity.
- **Interface size:** one ~50-method `Store` is a mild smell but matches sqlc's `Querier` and keeps the swap trivial; interface segregation (per-entity sub-interfaces) is a deferred, optional follow-up, not part of v0.1.0.
- **Group titles map:** `mongodb` stores group titles as a map (`titles.{id}`); `models.Group`/`models.GroupTitle` stay list-shaped as the service layer already consumes. The mapping handles map↔list (as the current service mappers already do).

### Delivery
Branch `v0.1.0-store-interface` off `main`; PR `v0.1.0 (1/4) — Store interface + neutral models`. No deploy (no runtime/behavior change). This is the first of the four v0.1.0 PRs.
