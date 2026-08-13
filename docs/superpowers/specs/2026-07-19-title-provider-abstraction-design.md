# Pluggable & Portable Title Provider (v0.0.9)

**Date:** 2026-07-19
**Status:** Approved design — ready for implementation planning

## Problem

The app enriches titles (movies/series) from an external metadata API. That API,
`api.imdbapi.dev`, has gone offline permanently: `api.imdbapi.dev` returns NXDOMAIN
from both the local resolver and public DNS (8.8.8.8), and the `imdbapi.dev` apex has
no A record. Every write/ingest path is broken as a result:

- `AddNewTitle` (add a movie/series by IMDb URL)
- `SearchTitles` (search by name)
- the cron refresh routine (`cmd/routines`)
- the test-fixture generator (`cmd/test-fixtures`)

Beyond the outage, the integration is **not portable**: the provider's wire format is
welded to the code. Specifically, in `AddNewTitle` the raw imdbapi.dev JSON is
`json.Unmarshal`ed **directly into `mongodb.TitleDb`**, so the provider's JSON shape IS
the database document shape. The `internal/imdb` package exposes five free functions
returning raw `[]byte`, and every caller unmarshals into `imdb.*` types shaped exactly
like imdbapi.dev's responses (and carrying `bson` tags). Swapping providers today would
mean rewriting all callers.

## Goal

Make the title metadata source **pluggable and fully portable**:

- Any supplier can be implemented behind one interface.
- The active supplier is chosen by configuration (env var) — no code change to switch.
- Migrate to **TMDB** (The Movie Database) now, since a working `TMDB_API_KEY` already
  exists in `.env`.
- If `imdbapi.dev` ever returns, switching back is just an env var change — the
  imdbapi.dev implementation is kept in the codebase behind the same interface.

## Non-goals

- No change to the MongoDB document shape (`mongodb.TitleDb`). Existing production data,
  the read path, and `tests/fixtures/*.json` stay untouched.
- No change to the public API contract or the frontend. Search still returns IMDb IDs;
  "add title" still works on IMDb IDs.
- No DB migration.

## Key decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Provider selection | Runtime env var `TITLE_PROVIDER` (`tmdb` \| `imdbapi`), both compiled in | Port back to imdbapi.dev with an env change + restart, no rebuild |
| Search bridging (TMDB has no IMDb IDs in results) | **Resolve-on-search**: provider resolves each result's `imdb_id` before returning | Keeps frontend + "add" flow unchanged; cost is N extra calls per search (N = limit, 2–5) |
| Canonical ID | IMDb ID (`tt...`) everywhere, as today | DB `_id`, URLs, fixtures all keyed on it; no migration |
| Domain model | New vendor-neutral types in `titleprovider`, no `bson` tags | True vendor neutrality (Approach A) |
| DB shape | Unchanged; map domain → `mongodb.TitleDb` explicitly | Preserve data, fixtures, read path |
| Default `TITLE_PROVIDER` | `tmdb` | imdbapi.dev is down |
| TMDB "Specials" (season 0) | Skip; ingest only numbered seasons ≥ 1 | Matches existing behavior expectations |
| Wiring | Dependency injection (no global singleton) | Testable, explicit |

## Architecture

### Package structure

```
internal/titleprovider/
    provider.go      # Provider interface + neutral domain types
    factory.go       # NewFromEnv() reads TITLE_PROVIDER -> returns a Provider
    imdbapi/         # imdbapi.dev implementation (current internal/imdb, refactored)
        imdbapi.go
    tmdb/            # NEW: TMDB implementation
        tmdb.go
```

`internal/imdb` is removed. Its HTTP logic moves into `titleprovider/imdbapi` and now
maps imdbapi.dev JSON into the neutral domain types instead of returning raw `[]byte`.

### The Provider interface

```go
type Provider interface {
    // GetTitle returns a fully-populated title by IMDb ID (tt...), including
    // seasons and episodes when it is a series. The provider hides all
    // multi-call orchestration (details + seasons + paginated episodes) internally.
    GetTitle(ctx context.Context, imdbID string) (*Title, error)

    // SearchTitles returns up to `limit` results, each identified by IMDb ID.
    SearchTitles(ctx context.Context, query string, limit int) ([]SearchItem, error)

    // Name returns the provider identifier, for logging.
    Name() string
}
```

This collapses today's five free functions (`FetchTitle`, `FetchSeasons`,
`FetchEpisodes`, `FetchBatchTitles`, `FetchTitlesBySearch`) into two methods. Seasons/
episodes pagination no longer leaks into callers — the provider owns it. The cron
refresh's "batch get" becomes a loop of `GetTitle` calls (TMDB has no batch endpoint).

### Neutral domain types

The domain types live in `titleprovider` and mirror today's `imdb.*` shapes but carry
**no `bson` tags** — pure vendor-neutral domain, with no storage concern:

`Title`, `Image`, `Person`, `Rating`, `CodeName`, `Interest`, `Season`, `Episode`,
`ReleaseDate`, `SearchItem`.

Fields with no TMDB equivalent (`Metacritic`, `Interests`) remain in the model but are
left empty/nil when the TMDB provider is active.

### Mapping to storage

The DB shape (`mongodb.TitleDb`) is unchanged. A new mapper
`titleprovider.Title -> mongodb.TitleDb` replaces the
`json.Unmarshal(body, &mongodb.TitleDb)` shortcut in `AddNewTitle`. The existing
`Map*` helpers in `internal/services/titles/mapper.go` that reference `imdb.*` are
updated to reference the new domain types (or are superseded by the new mapper).

## TMDB provider details

- **Base URL:** `https://api.themoviedb.org/3`
- **Auth:** `?api_key=<TMDB_API_KEY>` (v3 key; verified working — 32-char hex).
- **`GetTitle(ctx, tt)`**
  1. `GET /find/{tt}?external_source=imdb_id` → determines movie vs. tv and the TMDB
     numeric id. Empty results → `ErrTitleNotFound`.
  2. Movie: `GET /movie/{id}?append_to_response=credits`.
     TV: `GET /tv/{id}?append_to_response=credits,external_ids`, then for each numbered
     season (≥ 1) `GET /tv/{id}/season/{n}` to collect episodes.
  3. Map to domain `Title`.
- **`SearchTitles(ctx, query, limit)`**
  1. `GET /search/multi?query=...`.
  2. Keep `media_type` in {`movie`, `tv`}, up to `limit`.
  3. For each kept result, `GET .../external_ids` to obtain `imdb_id`
     (**resolve-on-search**); drop any result lacking an `imdb_id`. Log dropped count.
  4. Map to `SearchItem` (identified by IMDb ID).

### TMDB → domain field mapping

| Domain field | TMDB source |
|--------------|-------------|
| `ID` (tt...) | `/find` input · `external_ids.imdb_id` |
| `Type` | endpoint / `media_type`: movie→`"movie"`, tv→`"tvSeries"` |
| `PrimaryTitle` | `title` / `name` |
| `StartYear` | `release_date[:4]` / `first_air_date[:4]` |
| `RuntimeSeconds` | `runtime` × 60 (movie); TV `episode_run_time` often empty → 0 |
| `Genres` | `genres[].name` |
| `Rating` | `vote_average`, `vote_count` |
| `Plot` | `overview` |
| Directors/Writers/Stars | `credits.crew` (Director / Writing dept), `credits.cast` |
| `PrimaryImage.URL` | `https://image.tmdb.org/t/p/original` + `poster_path` (width/height = 0) |
| `SpokenLanguages` / `OriginCountries` | `spoken_languages` / `production_countries` (ISO code; name may equal code) |
| `Seasons` | `seasons[]` {season_number ≥ 1, episode_count} |
| `Episodes` | `/tv/{id}/season/{n}` → episodes[] (name, air_date, runtime, vote, still_path) |
| `Metacritic` | — (nil) |
| `Interests` | — (nil) |

## Provider selection & wiring

- `titleprovider.NewFromEnv()` reads `TITLE_PROVIDER` (default `tmdb`; also `imdbapi`)
  and returns the matching `Provider`. Unknown value → startup error.
- The `API` struct (`internal/api/api.go`) gains a `Provider titleprovider.Provider`
  field; `NewAPI(db, provider)` is updated. `main.go` builds the provider from env and
  passes it in.
- Service functions take the provider explicitly:
  - `AddNewTitle(db, provider, ctx, imdbID)` — called from `AddTitle` and
    `AddTitleToGroup` handlers, both `*API` methods with `api.Provider`.
  - `SearchTitles(provider, query, limit)`.
- `cmd/routines` and `cmd/test-fixtures` construct the provider from env directly and
  call `GetTitle`.

## Error handling

- `titleprovider.ErrTitleNotFound` for empty `/find` results or a 404 on details.
- Other non-2xx responses are wrapped with status + body, as today.
- The service maps not-found to the existing API error responses; other errors surface
  as today.

## Testing

- **New unit tests per provider** using `httptest.Server` with canned TMDB and
  imdbapi.dev JSON, asserting the domain mapping (movie, TV with seasons/episodes,
  search + external_ids resolution, not-found). This also satisfies the backlog TODO
  for unit tests on the title/episode calls.
- **Integration tests unchanged** — they seed titles from `tests/fixtures/*.json` via
  direct DB insert (`seedTitles` → `InsertMany`) and never call the live provider.
- Optionally, a fake `Provider` for any service-level test that needs one.

## Config & release

- `env.example` gains `TITLE_PROVIDER` (default `tmdb`) and `TMDB_API_KEY`.
- Release steps: update `CHANGELOG.md`, tag **`v0.0.9`** at merge (lightweight tag on
  the merge commit, matching the existing convention; note `v0.0.8` was tagged
  retroactively at `15c7d62`).

## Impacted files (summary)

- **New:** `internal/titleprovider/{provider.go,factory.go}`,
  `internal/titleprovider/imdbapi/imdbapi.go`, `internal/titleprovider/tmdb/tmdb.go`,
  provider unit tests.
- **Removed:** `internal/imdb/` (moved into `titleprovider/imdbapi`).
- **Changed:** `internal/services/titles/titles.go` (AddNewTitle, SearchTitles),
  `internal/services/titles/mapper.go`, `internal/api/api.go` (API struct + NewAPI),
  `main.go`, `cmd/routines/main.go`, `cmd/test-fixtures/main.go`, `env.example`,
  `CHANGELOG.md`.
