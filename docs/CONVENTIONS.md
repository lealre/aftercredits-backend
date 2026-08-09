# Backend code conventions

Conventions for the Go backend. New code should follow these; when touching old
code, nudge it toward them.

## 1. Handlers don't touch the database directly

HTTP handlers (`internal/api/*.go`) orchestrate request/response only. All data
access goes through a **service** (`internal/services/<domain>`), which owns the
DB calls. A handler should call `service.DoThing(db, ...)`, never
`api.Db.SomeQuery(...)` inline.

Why: keeps business logic and persistence testable and reusable, and keeps
handlers thin.

This is enforced: no handler in `internal/api` calls `api.Db.*` directly.
Existence/membership checks go through thin service passthroughs (e.g.
`titles.TitleExists`, `groups.GroupExists`, `groups.GroupContainsTitle`,
`users.UserExists`).

Prefer the cheapest guard that answers the question. `GroupExists` is a single
`EXISTS`; loading the group instead materializes every group title and every
season row, which is wasted work when only the answer matters.

## 2. Nothing outside the storage layer names a concrete database

Services and handlers depend on the **`store.Store`** interface
(`internal/store`) and the storage-neutral domain types in `internal/models` —
never on a concrete implementation package. `internal/postgres` is imported only
by the binaries' entrypoints (`main.go`, `cmd/database`, `cmd/routines`) and the
test harness, each of which builds the concrete store and passes it as a
`store.Store`. Nothing under `internal/services` or `internal/api` imports it.

- No storage type (sqlc rows, pgx types, SQL strings) appears in a
  `store.Store` method signature.
- Storage-specific errors are translated at the boundary into the shared
  sentinels `store.ErrRecordNotFound` and `store.ErrDuplicatedRecord`, so
  service-level error mapping stays database-agnostic.
- `internal/models` types carry no persistence tags.

Query construction belongs behind the interface. If a service needs a new
filter or sort, add an intent-revealing store method (e.g. `GetTitlesPage`)
rather than passing query fragments through.

## 3. Errors are defined in the service, mapped to HTTP in the handler

Each service package declares its error vocabulary as sentinel errors plus an
`ErrorMap` in `internal/services/<domain>/utils.go`:

```go
var ErrThing = errors.New("thing not found")
var ErrorMap = map[error]int{ ErrThing: http.StatusNotFound }
```

Handlers translate, they don't invent error strings:

```go
if code, ok := domain.ErrorMap[err]; ok {
    respondWithError(w, code, err.Error())
    return
}
// otherwise: log + 500
```

Error strings and their HTTP statuses live in one place (the service), not
scattered across handlers. Services translate lower-level errors (e.g. a
provider's `ErrTitleNotFound`) into their own vocabulary before returning.

**Check the error before the boolean.** For `(bool, error)` guards, a failed
query returns `(false, err)`, so testing `!ok` first reports an infrastructure
failure as `404 Not Found`:

```go
// wrong: a database error is reported as "not found"
if ok, err := groups.GroupExists(...); !ok { /* 404 */ } else if err != nil { /* 500 */ }

// right
if ok, err := groups.GroupExists(...); err != nil { /* 500 */ } else if !ok { /* 404 */ }
```

## 4. Comment invariants (movie vs. TV series)

A stored comment has a top-level `Comment` and a per-season `SeasonsComments`
map. Exactly one is used, by title type:

- **Movie:** top-level `Comment` is set; `SeasonsComments` MUST be empty/nil.
- **TV series:** top-level `Comment` MUST be empty/nil; `SeasonsComments` holds
  one entry per season.

The add/update flows enforce this by construction (separate movie vs. series
code paths). Don't write a path that populates both.

## 5. nil vs. empty is part of the API contract

Whether a field serializes as `null` or `[]`/`{}` is observable, and existing
clients depend on it. The rules the store upholds:

- A title read returns **non-nil, possibly empty** slices for `Directors`,
  `Writers`, `Stars`, `OriginCountries`, `SpokenLanguages`, `Interests`,
  `Seasons` and `Episodes`. `Genres` is passed through as stored.
- The per-season maps (`SeasonsRatings`, `SeasonsComments`, `SeasonsWatched`)
  are **nil** when the record has no season rows, never an empty map.
- A group's `Titles` map is always non-nil, possibly empty.

When changing a read path, assert the shape rather than assuming — a decoded Go
struct cannot distinguish `null` from `[]`, so tests that need that distinction
must assert on the raw response body.

## 6. Ordering must be total

Any sort that feeds pagination needs a tie-break, or the same request can
return different pages. Ids assembled by ranging a map start in an arbitrary
order, so a comparator that leaves ties unresolved is effectively random. Every
branch of a sort should end in a deterministic tie-break (by id).

## 7. Schema migrations

Migrations are goose files in `sql/schema/NNN_name.sql`, embedded into the
binary (`sql/embed.go`) and applied by `database -migrate` **as a deploy step**,
before the server starts. The running application never changes the schema:
that keeps a failed migration a failed deploy rather than a crash-looping
container, and keeps DDL rights out of the application's database role.

Already-applied migrations are not edited — add a new one instead.

`internal/database` is generated by sqlc from `sql/queries/*.sql`. Never edit it
by hand; change the `.sql` and re-run `sqlc generate`.

## 8. Tests

Integration tests live in `tests/` and run against a real Postgres
testcontainer. They are hermetic: titles are seeded straight into the database
from the committed JSON fixtures, and the server is built with a fake title
provider, so no network access or API keys are needed.

- Files are paired per entity: `X_test.go` holds **only** `func Test...`;
  `X_setup_test.go` holds **only** helpers. New tests for an existing entity go
  in that entity's existing pair — don't add a new file.
- Group related cases under one `TestX` with `t.Run("descriptive case", ...)`
  subtests, each starting with `resetDB(t)`.
- Do HTTP through setup helpers, not inline `http.NewRequest`. Two flavors
  exist: helpers that assert success and return the decoded body, and helpers
  returning `*http.Response` for tests that assert status codes themselves.
- Do database assertions through helpers too.
- Give `require` assertions a descriptive message.

A regression test must be **mutation-checked**: revert the fix, confirm the new
test fails, then restore it. A test that passes with and without the fix is
worse than no test.

## 9. The changelog is written by hand

`docs/CHANGELOG.md` is maintained manually, newest first. Entries describe what
changed **for someone using or operating the service** — not commit subjects.
Call out anything that requires action (a migration, a re-login, a config
change).

The repository previously carried a `git-chglog` configuration that regenerated
the file from commit messages. It was removed: it overwrote hand-written
entries, emitted merge commits as changelog lines, and silently dropped any
release that was never tagged.

## Related

- Title metadata providers: [../internal/titleprovider/README.md](../internal/titleprovider/README.md)
- Scheduled tasks and deployment: [../pi/README.md](../pi/README.md)
