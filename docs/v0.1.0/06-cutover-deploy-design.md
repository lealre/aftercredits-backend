# v0.1.0 (4/4) — Cutover + deploy (DESIGN)

**Status:** implemented (this PR).
**Repo:** `github.com/lealre/movies-backend`.
**Umbrella:** sub-project 4 of 4 in the Mongo→Postgres migration (v0.1.0). Sub-projects 1–2 merged (PRs #17, #18); sub-project 3 (data migration) is the open PR on `v0.1.0-data-migration`. This branch (`v0.1.0-cutover`) is stacked on it.

## Goal
The app, its deploy one-shot, its scheduled jobs, its backups, and its integration test suite all run on Postgres. Mongo remains in the codebase only as the migration tool's dependency, deleted in a follow-up PR after the production cutover succeeds.

## Locked decisions (from brainstorming)
1. **Stacked branch, separate PR.** `v0.1.0-cutover` off `v0.1.0-data-migration`; PR `v0.1.0 (4/4) — cutover + deploy` based on the 3/4 branch.
2. **Mongo code stays in this PR.** `internal/mongodb`, `internal/pgmigration`, `cmd/mongo-to-postgres`, and the mongo driver dependency remain so the merged code can execute the production cutover. A follow-up cleanup PR (after the Pi cutover) deletes them plus `cmd/dev-migrations`, `cmd/test-fixtures` (it imports `internal/mongodb`), the mongo compose service, `MONGO_*` env, and the temporary migration binary in the Docker image; that PR must also move (or convert to plain JSON) the `tests/` fixtures, which still decode through `mongodb.TitleDb` types.
3. **Pi deploy files live in the backend `pi/` folder.** The frontend repo's `docker-compose-pi.yaml` is NOT touched; `pi/docker-compose-pi.yaml` is the complete reference to copy over (applied to the frontend repo later on request).
4. **Pi Postgres data on USB storage**: bind mount `/mnt/usb_storage/aftercredits_postgres:/var/lib/postgresql/data` (same pattern as Mongo's `/mnt/usb_storage/aftercredits_db`).
5. **Frozen contract holds.** `internal/store/store.go` and `internal/models` unchanged. New capabilities needed only by internal tools (`UpdateTitle`, `ListTitleIds`) are exported methods on `*postgres.Store` outside the interface.

## Components

### 1. App wiring (`main.go`, `internal/server`)
- `main.go`: build a `pgxpool.Pool` from `POSTGRES_USER/PASSWORD/HOST/PORT/DB` env (defaults matching docker-compose: `aftercredits`×3, `localhost`, `5432`; `sslmode=disable`) via a new `postgres.Connect(ctx) (*pgxpool.Pool, error)` helper that pings before returning (mirrors `mongodb.Connect`'s env pattern). Pass `postgres.New(pool)` to the server.
- `internal/server`: `NewServer(st store.Store) (http.Handler, error)`, `NewServerWithProvider(st store.Store, provider titleprovider.Provider, secret string) http.Handler` (drops the `*mongo.Client` param and the internal `mongodb.NewDB` call), `AuthMiddleware(tokenSecret string, db store.Store)` (it only calls `GetUserById`). `ListenAndServe(st store.Store) error`.
- `internal/server/server_test.go` updated to the new signatures.
- The untracked local `cmd/pg-preview` becomes obsolete and is removed from the working tree (it was never committed).

### 2. Deploy one-shot (`cmd/database`)
- Drop `-indexes` / `-reset` / `-delete` / `-backfill-groups` (Mongo-specific; the mongodb index helpers stay in `internal/mongodb`, unused, until the cleanup PR).
- Add `-migrate`: runs the embedded goose migrations. Embedding: new file `sql/embed.go` (`package sqlassets`) with `//go:embed schema/*.sql` exposing `SchemaFS embed.FS`; `cmd/database` does `goose.SetBaseFS(sqlassets.SchemaFS)` + `goose.SetDialect("postgres")` + `goose.Up(db, "schema")` over a pgx-stdlib `*sql.DB`.
- Keep `-superuser`: same env-driven creation (`SUPERUSER_*`), unchanged service path, now on the Postgres store.
- Binary name/path stay `/app/database`; the Pi compose `db-setup` command becomes `/app/database -migrate && /app/database -superuser`.

### 3. Scheduled title sync (`cmd/routines`)
- Port off raw Mongo access: `postgres.Connect` → `postgres.New`.
- `ListTitleIds(ctx) ([]string, error)` — new exported method on `*postgres.Store` (`SELECT id FROM titles ORDER BY id`, one new sqlc query).
- Per title: `GetTitleById` → apply the existing per-field refresh (primaryImage, seasons, episodes, rating, metacritic — comparing provider values against the stored model, same change-detection semantics) → `UpdateTitle(ctx, models.Title) error` — new exported method on `*postgres.Store` (one new sqlc query `UpdateTitle` that rewrites the denormalized columns + full JSONB metadata via the existing `titleToRow` params; `updated_at` set to the refresh time exactly as the Mongo version stamped it).
- Same 5-worker pool and logging; `pi/Dockerfile.routines` needs no change (same build path).
- The Mongo-specific mapping helpers it used (`titles.MapImdbSeasonsToDbSeasons` etc.) — the services/titles package maps provider→model today for the API path; routines reuses the same provider→model mapping (no mongodb types in the new code).

### 4. Integration tests (`tests/`)
- `setup_test.go`: postgres:16 testcontainer + goose migrations (same harness pattern as `internal/postgres`/`internal/pgmigration`), `server.NewServerWithProvider(postgres.New(pool), fakeProvider, "test-secret")`; `resetDB` = TRUNCATE all 10 tables.
- The `*_setup_test.go` DB helpers (raw `Collection`/`bson` seeds and reads) are reimplemented over the Postgres store methods and, where a raw row read is needed, the sqlc `database.Queries`. Test flows and assertions (the HTTP suite) stay semantically identical — the suite that specified Mongo behavior now proves the Postgres wiring.

### 5. Backups → pg_dump
- `scripts/backup.sh`: `docker exec aftercredits-postgres pg_dump -U … -Fc` → `backups/pg_dump_<ts>.dump` + `.tar.gz` wrap (keeps the tar.gz-artifact convention). `scripts/restore.sh`: `pg_restore --clean --if-exists`.
- `pi/backup_to_drive.sh`: same flow with `pg_dump` inside the backup container; `pi/Dockerfile.backup`: `postgresql16-client` replaces `mongodb-tools`. Artifact naming `pg_dump_<ts>.tar.gz`; rclone target unchanged.

### 6. Pi deploy files (`pi/`)
- `pi/docker-compose-pi.yaml` (new, reference for the frontend repo's file): `postgres` service (postgres:16, bind mount per decision 4, healthcheck `pg_isready`), `db-setup` (image `lealre/aftercredits-backend:latest`, new command, `service_healthy` dependency), `backend` (unchanged apart from depending on postgres), `frontend` (unchanged).
- `pi/.env.example`: `POSTGRES_*` block replaces the Mongo block for the app/backup; `MONGO_*` kept in a clearly-marked "cutover only" section (the one-time migration still needs to reach the old Mongo/backup).
- `pi/README.md`: updated setup + a **cutover runbook**: take final mongodump backup → stop backend → start postgres service → run `db-setup` (`-migrate`) → run the migration (`docker run … lealre/aftercredits-backend:latest /app/mongo-to-postgres -dump …`) → verification must exit 0 → start the new stack → smoke → leave Mongo container stopped-but-intact as rollback until the cleanup PR.
- `Dockerfile` / `Dockerfile.push`: additionally build `/app/mongo-to-postgres` (temporary, removed in the cleanup PR) so the Pi can run the cutover without a Go toolchain.

### 7. Local dev compose + env
- Root `docker-compose.yaml`: postgres service stays primary; mongo service kept (needed only to restore old backups / run the migration) with a comment marking it cutover-legacy.
- `env.example`: `POSTGRES_*` block added as the app's database config; `MONGO_*` kept, marked as legacy (migration tool only). README run instructions updated.

## Non-goals
No `store.Store`/`internal/models` changes; no API or response changes; no frontend code changes (compose reference only); no Mongo code deletion (follow-up PR); no v0.1.0 tag (after the real cutover).

## Testing / acceptance criteria
- Full `tests/` HTTP suite green against the Postgres-wired server (testcontainers).
- `internal/postgres`, `internal/pgmigration`, `internal/mongodb` unit/integration suites still green.
- `go build ./... && go vet ./...` clean; `gofmt` clean (module: only the pre-existing violation); `go` directive stays `1.24.0`; no new module dependencies.
- `sqlc generate` is a no-op after the new queries are committed.
- `internal/store` + `internal/models` diffs vs main: empty.
- `cmd/database -migrate` applies cleanly to an empty Postgres (exercised by the tests' goose path and a manual smoke); `-superuser` creates the superuser from env.
- `cmd/routines` compiles and its refresh logic is covered by a store-level test for `UpdateTitle`/`ListTitleIds`.
- Local end-to-end smoke: real `main.go` + compose postgres (migrated data) + frontend dev server — replaces the pg-preview.

## Risks / notes
- `AuthMiddleware`/`NewServer` signature changes ripple into `tests/` and `internal/server/server_test.go` — all in this PR, nothing external imports them.
- `UpdateTitle` must preserve the metadata JSONB convention (write through `titleToRow`) or read-fidelity breaks.
- The tests port is the largest mechanical surface (6 setup files); behavior must not be "adjusted to pass" — where Postgres output legitimately differs, that's a wiring bug, not a test to change.
- Image size: one extra static binary (`mongo-to-postgres`), temporary.

## Delivery
Branch `v0.1.0-cutover` (stacked); PR `v0.1.0 (4/4) — cutover + deploy` based on `v0.1.0-data-migration`. Curated doc copies (`docs/v0.1.0/06-cutover-deploy-design.md`) in the PR. Follow-up cleanup PR after the production cutover.
