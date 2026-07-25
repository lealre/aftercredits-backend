# Title metadata providers

Title metadata (name, year, runtime, genres, plot, poster, cast, ratings, and —
for series — seasons/episodes) comes from an external source behind the
`titleprovider.Provider` interface. The active source is chosen at startup by the
`TITLE_PROVIDER` env var, so a supplier can be swapped without code changes.

Canonical IDs are always **IMDb IDs** (`tt...`) — DB `_id`, URLs, and fixtures all
key on them, and every provider takes/returns them.

## Which provider is good at what

| Provider | Ratings | Seasons / Episodes | Search | Extra data | Key needed | Rate limit |
|----------|---------|--------------------|--------|------------|------------|------------|
| **hybrid** (recommended) | ✅ **IMDb** rating + votes (from OMDb) | ✅ rich (from TMDB: images, plot, runtime) | ✅ via TMDB | ✅ Metacritic (OMDb) | TMDB + OMDb | TMDB ~50/s · OMDb 1k/day |
| **tmdb** | ⚠️ TMDB's *own* community rating (not IMDb) | ✅ rich (images, plot, runtime, per-episode) | ✅ resolve-on-search to `tt` | — | TMDB | ~50/s |
| **omdb** | ✅ **IMDb** rating + votes | ⚠️ episode list only (no images/plot/runtime; extra calls) | ✅ native `tt` results | ✅ Metacritic | OMDb | 1,000/day |
| **imdbapi** | ✅ IMDb rating | ✅ rich | ✅ | metacritic, interests | none | — (**offline**) |

### hybrid — TMDB metadata + OMDb IMDb rating (default deployment)
- **Good:** best of both — TMDB's rich seasons/episodes/images **and** the real
  IMDb rating + vote count (overlaid onto the title), plus Metacritic. One extra
  OMDb call per `GetTitle`.
- **Weak:** needs two API keys. If OMDb is momentarily down it degrades
  gracefully to TMDB's rating (never blocks add/refresh). **Search results** carry
  TMDB's rating for previewing candidates; the authoritative IMDb rating is
  applied when the title is actually added. **Episode ratings** stay TMDB's (only
  the title-level rating is overlaid, to keep within OMDb's daily limit).

### tmdb — The Movie Database
- **Good:** excellent seasons/episodes with still images, plots, per-episode
  runtime; generous rate limit; great posters.
- **Weak:** its rating is TMDB's own community score (e.g. *Michael* 8.7) which
  **differs from IMDb** (7.4), and comes with extra decimals. No Metacritic. Needs
  a `/find` + `external_ids` round-trip because TMDB uses its own numeric IDs.

### omdb — The Open Movie Database
- **Good:** returns the **real IMDb rating** and vote count directly, plus
  Metacritic; keyed natively by `tt` ID; search returns `tt` IDs with no extra
  lookups; also exposes rated/released/country/language.
- **Weak:** episode data is thin — a per-season episode list (number, title, air
  date, per-episode IMDb rating) but **no episode still-images, plot, or runtime**
  without extra per-episode calls. Free tier is **1,000 requests/day**, and a full
  library refresh of many long series can approach it.

### imdbapi — api.imdbapi.dev (legacy)
- **Good:** was the original source; rich data, IMDb ratings, no key.
- **Weak:** **the service is offline** (DNS NXDOMAIN since ~July 2026). Kept behind
  the interface only so it can be re-enabled instantly (`TITLE_PROVIDER=imdbapi`)
  if it ever returns.

## Configuration

```bash
TITLE_PROVIDER=hybrid      # hybrid | tmdb | omdb | imdbapi   (default: tmdb)
TMDB_API_KEY=...           # required for hybrid and tmdb (v3 key)
OMDB_API_KEY=...           # required for hybrid and omdb (free: omdbapi.com/apikey.aspx)
```

## Adding a new provider

Implement `titleprovider.Provider` (`GetTitle`, `SearchTitles`, `Name`) in a new
subpackage, map the vendor's payload into the neutral `titleprovider` domain
types, then add a case to `factory.NewFromEnv`. Keep vendor wire structs private
to the subpackage. Add a row to the table above.
