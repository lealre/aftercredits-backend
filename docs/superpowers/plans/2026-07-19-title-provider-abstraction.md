# Pluggable & Portable Title Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hard dependency on `api.imdbapi.dev` with a pluggable title-metadata provider selected by an env var, implement a TMDB provider, and keep the imdbapi.dev provider for easy port-back.

**Architecture:** A new `internal/titleprovider` package defines a `Provider` interface plus vendor-neutral domain types. Two implementations (`imdbapi`, `tmdb`) each own their HTTP calls and map their wire format into the domain types. A factory picks one from `TITLE_PROVIDER`. The chosen provider is dependency-injected into the API and CLIs. The MongoDB document shape (`mongodb.TitleDb`) is unchanged — a new mapper converts a domain `Title` into it, replacing the current `json.Unmarshal(body, &TitleDb)` shortcut.

**Tech Stack:** Go 1.24, standard `net/http` + `encoding/json`, MongoDB driver, `testify`, `httptest` for provider unit tests.

## Global Constraints

- Module path: `github.com/lealre/movies-backend`. Go version floor: `go 1.24.0`.
- Canonical title ID is the IMDb ID (`tt...`) everywhere: DB `_id`, URLs, fixtures. Never change this.
- Do NOT change the MongoDB document shape (`mongodb.TitleDb` and its nested types in `internal/mongodb/titles_db.go`). Existing production data, the read path, and `tests/fixtures/*.json` depend on it.
- Do NOT regenerate `tests/fixtures/*.json`. They contain imdbapi.dev-shaped data that existing tests assert against; regenerating from TMDB would change values and break tests.
- Domain types in `titleprovider` carry NO `json`/`bson` tags — nothing serializes them directly. Each provider maps its own wire structs into them; the service maps them into `mongodb.TitleDb`.
- Default `TITLE_PROVIDER` value is `tmdb`. Allowed values: `tmdb`, `imdbapi`. Unknown value → startup error.
- TMDB auth: v3 API key passed as `?api_key=<TMDB_API_KEY>`.
- TMDB "Specials" (season_number 0) are skipped; only numbered seasons ≥ 1 are ingested.
- Type strings: TMDB movie → `"movie"`, TMDB tv → `"tvSeries"` (so the existing `type == "tvSeries" || type == "tvMiniSeries"` series check triggers season/episode fetching).
- Release tag for this work: `v0.0.9`, lightweight tag on the merge commit (matching convention).

---

## File Structure

**Create:**
- `internal/titleprovider/provider.go` — `Provider` interface, domain types, `ErrTitleNotFound`.
- `internal/titleprovider/factory/factory.go` — `NewFromEnv()`. **Separate package** (`factory`) to avoid an import cycle: the `tmdb`/`imdbapi` subpackages import `titleprovider` for the domain types, so the factory that imports those subpackages cannot itself live in `titleprovider`.
- `internal/titleprovider/factory/factory_test.go`
- `internal/titleprovider/imdbapi/imdbapi.go` — imdbapi.dev provider + wire types + mapping.
- `internal/titleprovider/imdbapi/imdbapi_test.go`
- `internal/titleprovider/tmdb/tmdb.go` — TMDB provider + wire types + mapping.
- `internal/titleprovider/tmdb/tmdb_test.go`
- `internal/services/titles/provider_mapper.go` — domain `Title` → `mongodb.TitleDb`, domain search → `[]Title`.
- `internal/services/titles/provider_mapper_test.go`

**Modify:**
- `internal/api/api.go` — add `Provider` field, update `NewAPI`.
- `internal/server/server.go` — build provider, pass to `NewAPI`.
- `main.go` — no change needed (server owns wiring) — verified below.
- `internal/services/titles/titles.go` — `AddNewTitle` / `SearchTitles` take a provider, use domain types + new mapper.
- `internal/services/titles/mapper.go` — retype the `MapImdb*` helpers to accept `titleprovider` types (used by routines) or remove if superseded.
- `internal/api/titles_handlers.go` — pass `api.Provider` to the service calls.
- `internal/api/groups_handler.go:244` — pass `api.Provider` to `AddNewTitle`.
- `cmd/routines/main.go` — use `provider.GetTitle`.
- `cmd/test-fixtures/main.go` — use provider + new mapper (kept compiling; not re-run).
- `tests/titles_setup_test.go` — `seedTitles` reads `[]mongodb.TitleDb`.
- `env.example` — add `TITLE_PROVIDER`, `TMDB_API_KEY`.
- `CHANGELOG.md` — v0.0.9 entry.

**Remove (final task only):**
- `internal/imdb/imdb.go`, `internal/imdb/types.go`.

---

### Task 1: Domain types + Provider interface

**Files:**
- Create: `internal/titleprovider/provider.go`
- Test: `internal/titleprovider/provider_test.go`

**Interfaces:**
- Produces: the `Provider` interface and domain types consumed by every later task:
  - `Provider.GetTitle(ctx context.Context, imdbID string) (*Title, error)`
  - `Provider.SearchTitles(ctx context.Context, query string, limit int) ([]SearchItem, error)`
  - `Provider.Name() string`
  - `ErrTitleNotFound error`
  - Domain structs: `Title, Image, Person, Rating, Metacritic, CodeName, Interest, Season, Episode, ReleaseDate, SearchItem`.

- [ ] **Step 1: Write the failing test**

```go
// internal/titleprovider/provider_test.go
package titleprovider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

// fakeProvider verifies the interface is implementable with the expected signatures.
type fakeProvider struct{}

func (fakeProvider) GetTitle(_ context.Context, imdbID string) (*titleprovider.Title, error) {
	if imdbID == "missing" {
		return nil, titleprovider.ErrTitleNotFound
	}
	return &titleprovider.Title{ID: imdbID, Type: "movie", PrimaryTitle: "X"}, nil
}
func (fakeProvider) SearchTitles(_ context.Context, _ string, _ int) ([]titleprovider.SearchItem, error) {
	return []titleprovider.SearchItem{{ID: "tt1", PrimaryTitle: "X"}}, nil
}
func (fakeProvider) Name() string { return "fake" }

func TestProviderInterfaceShape(t *testing.T) {
	var p titleprovider.Provider = fakeProvider{}

	got, err := p.GetTitle(context.Background(), "tt0068646")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "tt0068646" {
		t.Fatalf("got ID %q", got.ID)
	}

	_, err = p.GetTitle(context.Background(), "missing")
	if !errors.Is(err, titleprovider.ErrTitleNotFound) {
		t.Fatalf("expected ErrTitleNotFound, got %v", err)
	}

	items, err := p.SearchTitles(context.Background(), "x", 5)
	if err != nil || len(items) != 1 {
		t.Fatalf("search failed: %v items=%d", err, len(items))
	}
	if p.Name() != "fake" {
		t.Fatalf("name %q", p.Name())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/titleprovider/ -run TestProviderInterfaceShape`
Expected: FAIL — package/types not defined (build error).

- [ ] **Step 3: Write the implementation**

```go
// internal/titleprovider/provider.go
package titleprovider

import (
	"context"
	"errors"
)

// ErrTitleNotFound is returned when a title cannot be found for the given IMDb ID.
var ErrTitleNotFound = errors.New("title not found")

// Provider is the vendor-neutral interface every title metadata source implements.
type Provider interface {
	// GetTitle returns a fully-populated title by IMDb ID (tt...), including
	// seasons and episodes when it is a series. Implementations hide all
	// multi-call orchestration internally. Returns ErrTitleNotFound if absent.
	GetTitle(ctx context.Context, imdbID string) (*Title, error)

	// SearchTitles returns up to limit results, each identified by IMDb ID.
	SearchTitles(ctx context.Context, query string, limit int) ([]SearchItem, error)

	// Name returns the provider identifier, for logging.
	Name() string
}

// Title is the vendor-neutral representation of a movie or series.
type Title struct {
	ID              string
	Type            string // "movie" | "tvSeries" | "tvMiniSeries"
	PrimaryTitle    string
	PrimaryImage    Image
	StartYear       int
	RuntimeSeconds  int
	Genres          []string
	Rating          Rating
	Metacritic      *Metacritic // nil when the provider has no metacritic data
	Plot            string
	Directors       []Person
	Writers         []Person
	Stars           []Person
	OriginCountries []CodeName
	SpokenLanguages []CodeName
	Interests       []Interest // empty when the provider has no equivalent
	Seasons         []Season
	Episodes        []Episode
}

type Image struct {
	URL    string
	Width  int
	Height int
}

type Person struct {
	ID                 string
	DisplayName        string
	AlternativeNames   []string
	PrimaryImage       *Image
	PrimaryProfessions []string
}

type Rating struct {
	AggregateRating float64
	VoteCount       int
}

type Metacritic struct {
	Score       int
	ReviewCount int
}

type CodeName struct {
	Code string
	Name string
}

type Interest struct {
	ID         string
	Name       string
	IsSubgenre bool
}

type Season struct {
	Season       string
	EpisodeCount int
}

type Episode struct {
	ID             string
	Title          string
	PrimaryImage   Image
	Season         string
	EpisodeNumber  int
	RuntimeSeconds *int
	Plot           *string
	Rating         *Rating
	ReleaseDate    *ReleaseDate
}

type ReleaseDate struct {
	Year  int
	Month int
	Day   int
}

// SearchItem is a single search result, identified by IMDb ID.
type SearchItem struct {
	ID           string
	Type         string
	PrimaryTitle string
	PrimaryImage Image
	StartYear    int
	Rating       Rating
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/titleprovider/ -run TestProviderInterfaceShape`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/titleprovider/provider.go internal/titleprovider/provider_test.go
git commit -m "feat(titleprovider): add Provider interface and neutral domain types"
```

---

### Task 2: TMDB provider

**Files:**
- Create: `internal/titleprovider/tmdb/tmdb.go`
- Test: `internal/titleprovider/tmdb/tmdb_test.go`

**Interfaces:**
- Consumes: `titleprovider` domain types + `ErrTitleNotFound` (Task 1).
- Produces:
  - `tmdb.New(apiKey string) *tmdb.Provider`
  - `(*tmdb.Provider)` implements `titleprovider.Provider`.
  - Internal test seam: `newWithBaseURL(baseURL, apiKey string) *Provider`.

- [ ] **Step 1: Write the failing test**

```go
// internal/titleprovider/tmdb/tmdb_test.go
package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

func newTestServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range routes {
		b := body
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(b))
		})
	}
	return httptest.NewServer(mux)
}

func TestGetTitle_Movie(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/find/tt0068646": `{"movie_results":[{"id":238}],"tv_results":[]}`,
		"/movie/238": `{"id":238,"title":"The Godfather","overview":"Plot.",
			"release_date":"1972-03-14","runtime":175,"poster_path":"/p.jpg",
			"vote_average":8.7,"vote_count":100,"genres":[{"name":"Drama"},{"name":"Crime"}],
			"spoken_languages":[{"iso_639_1":"en","english_name":"English"}],
			"production_countries":[{"iso_3166_1":"US","name":"United States"}],
			"credits":{"cast":[{"id":1,"name":"Marlon Brando","profile_path":"/b.jpg"}],
			"crew":[{"id":2,"name":"Francis Ford Coppola","job":"Director","department":"Directing"},
			{"id":3,"name":"Mario Puzo","job":"Writer","department":"Writing"}]},
			"external_ids":{"imdb_id":"tt0068646"}}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	got, err := p.GetTitle(context.Background(), "tt0068646")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.ID != "tt0068646" || got.Type != "movie" {
		t.Fatalf("ID/Type: %q %q", got.ID, got.Type)
	}
	if got.PrimaryTitle != "The Godfather" {
		t.Fatalf("title %q", got.PrimaryTitle)
	}
	if got.StartYear != 1972 {
		t.Fatalf("year %d", got.StartYear)
	}
	if got.RuntimeSeconds != 175*60 {
		t.Fatalf("runtime %d", got.RuntimeSeconds)
	}
	if got.PrimaryImage.URL != "https://image.tmdb.org/t/p/original/p.jpg" {
		t.Fatalf("image %q", got.PrimaryImage.URL)
	}
	if len(got.Genres) != 2 || got.Genres[0] != "Drama" {
		t.Fatalf("genres %v", got.Genres)
	}
	if got.Rating.AggregateRating != 8.7 || got.Rating.VoteCount != 100 {
		t.Fatalf("rating %+v", got.Rating)
	}
	if len(got.Directors) != 1 || got.Directors[0].DisplayName != "Francis Ford Coppola" {
		t.Fatalf("directors %+v", got.Directors)
	}
	if len(got.Writers) != 1 || got.Writers[0].DisplayName != "Mario Puzo" {
		t.Fatalf("writers %+v", got.Writers)
	}
	if len(got.Stars) != 1 || got.Stars[0].DisplayName != "Marlon Brando" {
		t.Fatalf("stars %+v", got.Stars)
	}
	if got.Metacritic != nil {
		t.Fatalf("metacritic should be nil")
	}
}

func TestGetTitle_TVWithSeasons(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/find/tt0903747": `{"movie_results":[],"tv_results":[{"id":1396}]}`,
		"/tv/1396": `{"id":1396,"name":"Breaking Bad","overview":"Plot.",
			"first_air_date":"2008-01-20","episode_run_time":[47],"poster_path":"/bb.jpg",
			"vote_average":8.9,"vote_count":200,"genres":[{"name":"Drama"}],
			"external_ids":{"imdb_id":"tt0903747"},
			"seasons":[{"season_number":0,"episode_count":9},{"season_number":1,"episode_count":2}]}`,
		"/tv/1396/season/1": `{"season_number":1,"episodes":[
			{"id":10,"name":"Pilot","episode_number":1,"season_number":1,"overview":"o1",
			 "air_date":"2008-01-20","runtime":59,"still_path":"/s1.jpg","vote_average":8.5,"vote_count":50},
			{"id":11,"name":"Cat","episode_number":2,"season_number":1,"overview":"o2",
			 "air_date":"2008-01-27","runtime":49,"still_path":"/s2.jpg","vote_average":8.2,"vote_count":40}]}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	got, err := p.GetTitle(context.Background(), "tt0903747")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.Type != "tvSeries" {
		t.Fatalf("type %q", got.Type)
	}
	// Season 0 (Specials) is skipped.
	if len(got.Seasons) != 1 || got.Seasons[0].Season != "1" || got.Seasons[0].EpisodeCount != 2 {
		t.Fatalf("seasons %+v", got.Seasons)
	}
	if len(got.Episodes) != 2 {
		t.Fatalf("episodes %d", len(got.Episodes))
	}
	e := got.Episodes[0]
	if e.Season != "1" || e.EpisodeNumber != 1 || e.Title != "Pilot" {
		t.Fatalf("ep0 %+v", e)
	}
	if e.RuntimeSeconds == nil || *e.RuntimeSeconds != 59*60 {
		t.Fatalf("ep runtime %+v", e.RuntimeSeconds)
	}
	if e.ReleaseDate == nil || e.ReleaseDate.Year != 2008 || e.ReleaseDate.Month != 1 || e.ReleaseDate.Day != 20 {
		t.Fatalf("ep releasedate %+v", e.ReleaseDate)
	}
	if e.Rating == nil || e.Rating.AggregateRating != 8.5 {
		t.Fatalf("ep rating %+v", e.Rating)
	}
}

func TestGetTitle_NotFound(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/find/tt0000000": `{"movie_results":[],"tv_results":[]}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	_, err := p.GetTitle(context.Background(), "tt0000000")
	if !errors.Is(err, titleprovider.ErrTitleNotFound) {
		t.Fatalf("expected ErrTitleNotFound, got %v", err)
	}
}

func TestSearchTitles_ResolvesImdbID(t *testing.T) {
	srv := newTestServer(t, map[string]string{
		"/search/multi": `{"results":[
			{"id":238,"media_type":"movie","title":"The Godfather","release_date":"1972-03-14",
			 "poster_path":"/p.jpg","vote_average":8.7,"vote_count":100},
			{"id":999,"media_type":"person","name":"Someone"},
			{"id":1396,"media_type":"tv","name":"Breaking Bad","first_air_date":"2008-01-20",
			 "poster_path":"/bb.jpg","vote_average":8.9,"vote_count":200}]}`,
		"/movie/238/external_ids": `{"imdb_id":"tt0068646"}`,
		"/tv/1396/external_ids":   `{"imdb_id":"tt0903747"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	items, err := p.SearchTitles(context.Background(), "godfather", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// person dropped; two results with resolved imdb ids.
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "tt0068646" || items[0].Type != "movie" || items[0].StartYear != 1972 {
		t.Fatalf("item0 %+v", items[0])
	}
	if items[1].ID != "tt0903747" || items[1].Type != "tvSeries" {
		t.Fatalf("item1 %+v", items[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/titleprovider/tmdb/`
Expected: FAIL — `tmdb` package/functions not defined.

- [ ] **Step 3: Write the implementation**

```go
// internal/titleprovider/tmdb/tmdb.go
package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

const (
	defaultBaseURL = "https://api.themoviedb.org/3"
	imageBaseURL   = "https://image.tmdb.org/t/p/original"
)

// Provider implements titleprovider.Provider against The Movie Database (TMDB) v3.
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New returns a TMDB provider using the public v3 base URL.
func New(apiKey string) *Provider {
	return &Provider{baseURL: defaultBaseURL, apiKey: apiKey, client: http.DefaultClient}
}

// newWithBaseURL is the test seam; it points the provider at a local server.
func newWithBaseURL(baseURL, apiKey string) *Provider {
	return &Provider{baseURL: baseURL, apiKey: apiKey, client: http.DefaultClient}
}

func (p *Provider) Name() string { return "tmdb" }

// getJSON performs a GET with the api_key query param and decodes JSON into out.
func (p *Provider) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", p.apiKey)
	reqURL := fmt.Sprintf("%s%s?%s", p.baseURL, path, query.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tmdb: non-2xx status %s for %s - %s", resp.Status, path, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	var find findResponse
	q := url.Values{}
	q.Set("external_source", "imdb_id")
	if err := p.getJSON(ctx, "/find/"+imdbID, q, &find); err != nil {
		return nil, err
	}

	switch {
	case len(find.MovieResults) > 0:
		return p.getMovie(ctx, imdbID, find.MovieResults[0].ID)
	case len(find.TVResults) > 0:
		return p.getTV(ctx, imdbID, find.TVResults[0].ID)
	default:
		return nil, titleprovider.ErrTitleNotFound
	}
}

func (p *Provider) getMovie(ctx context.Context, imdbID string, tmdbID int) (*titleprovider.Title, error) {
	var d movieDetails
	q := url.Values{}
	q.Set("append_to_response", "credits")
	if err := p.getJSON(ctx, "/movie/"+strconv.Itoa(tmdbID), q, &d); err != nil {
		return nil, err
	}

	t := &titleprovider.Title{
		ID:              imdbID,
		Type:            "movie",
		PrimaryTitle:    d.Title,
		PrimaryImage:    imageOf(d.PosterPath),
		StartYear:       yearFromDate(d.ReleaseDate),
		RuntimeSeconds:  d.Runtime * 60,
		Genres:          genreNames(d.Genres),
		Rating:          titleprovider.Rating{AggregateRating: d.VoteAverage, VoteCount: d.VoteCount},
		Plot:            d.Overview,
		Directors:       crewByJob(d.Credits.Crew, "Director"),
		Writers:         crewByDepartment(d.Credits.Crew, "Writing"),
		Stars:           castToPersons(d.Credits.Cast),
		OriginCountries: countryCodeNames(d.ProductionCountries),
		SpokenLanguages: languageCodeNames(d.SpokenLanguages),
	}
	return t, nil
}

func (p *Provider) getTV(ctx context.Context, imdbID string, tmdbID int) (*titleprovider.Title, error) {
	var d tvDetails
	q := url.Values{}
	q.Set("append_to_response", "credits,external_ids")
	if err := p.getJSON(ctx, "/tv/"+strconv.Itoa(tmdbID), q, &d); err != nil {
		return nil, err
	}

	runtime := 0
	if len(d.EpisodeRunTime) > 0 {
		runtime = d.EpisodeRunTime[0] * 60
	}

	t := &titleprovider.Title{
		ID:              imdbID,
		Type:            "tvSeries",
		PrimaryTitle:    d.Name,
		PrimaryImage:    imageOf(d.PosterPath),
		StartYear:       yearFromDate(d.FirstAirDate),
		RuntimeSeconds:  runtime,
		Genres:          genreNames(d.Genres),
		Rating:          titleprovider.Rating{AggregateRating: d.VoteAverage, VoteCount: d.VoteCount},
		Plot:            d.Overview,
		Directors:       crewByJob(d.Credits.Crew, "Director"),
		Writers:         crewByDepartment(d.Credits.Crew, "Writing"),
		Stars:           castToPersons(d.Credits.Cast),
		OriginCountries: countryCodeNames(d.ProductionCountries),
		SpokenLanguages: languageCodeNames(d.SpokenLanguages),
	}

	for _, s := range d.Seasons {
		if s.SeasonNumber < 1 { // skip Specials (season 0)
			continue
		}
		t.Seasons = append(t.Seasons, titleprovider.Season{
			Season:       strconv.Itoa(s.SeasonNumber),
			EpisodeCount: s.EpisodeCount,
		})

		var sd tmdbSeasonDetails
		if err := p.getJSON(ctx, fmt.Sprintf("/tv/%d/season/%d", tmdbID, s.SeasonNumber), nil, &sd); err != nil {
			return nil, err
		}
		for _, e := range sd.Episodes {
			t.Episodes = append(t.Episodes, mapEpisode(e))
		}
	}

	return t, nil
}

func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	var resp searchMultiResponse
	q := url.Values{}
	q.Set("query", query)
	if err := p.getJSON(ctx, "/search/multi", q, &resp); err != nil {
		return nil, err
	}

	items := make([]titleprovider.SearchItem, 0, limit)
	for _, r := range resp.Results {
		if len(items) >= limit {
			break
		}
		if r.MediaType != "movie" && r.MediaType != "tv" {
			continue
		}

		var ext tmdbExternalIDs
		path := fmt.Sprintf("/movie/%d/external_ids", r.ID)
		if r.MediaType == "tv" {
			path = fmt.Sprintf("/tv/%d/external_ids", r.ID)
		}
		if err := p.getJSON(ctx, path, nil, &ext); err != nil {
			return nil, err
		}
		if ext.IMDbID == "" { // drop titles with no linked IMDb ID
			continue
		}

		item := titleprovider.SearchItem{
			ID:           ext.IMDbID,
			PrimaryImage: imageOf(r.PosterPath),
			Rating:       titleprovider.Rating{AggregateRating: r.VoteAverage, VoteCount: r.VoteCount},
		}
		if r.MediaType == "movie" {
			item.Type = "movie"
			item.PrimaryTitle = r.Title
			item.StartYear = yearFromDate(r.ReleaseDate)
		} else {
			item.Type = "tvSeries"
			item.PrimaryTitle = r.Name
			item.StartYear = yearFromDate(r.FirstAirDate)
		}
		items = append(items, item)
	}
	return items, nil
}

// ----- mapping helpers -----

func imageOf(path string) titleprovider.Image {
	if path == "" {
		return titleprovider.Image{}
	}
	return titleprovider.Image{URL: imageBaseURL + path}
}

func yearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

func genreNames(gs []tmdbGenre) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

func countryCodeNames(cs []tmdbCountry) []titleprovider.CodeName {
	out := make([]titleprovider.CodeName, 0, len(cs))
	for _, c := range cs {
		out = append(out, titleprovider.CodeName{Code: c.ISO, Name: c.Name})
	}
	return out
}

func languageCodeNames(ls []tmdbLang) []titleprovider.CodeName {
	out := make([]titleprovider.CodeName, 0, len(ls))
	for _, l := range ls {
		name := l.Name
		if name == "" {
			name = l.ISO
		}
		out = append(out, titleprovider.CodeName{Code: l.ISO, Name: name})
	}
	return out
}

func castToPersons(cast []tmdbCast) []titleprovider.Person {
	out := make([]titleprovider.Person, 0, len(cast))
	for _, c := range cast {
		out = append(out, titleprovider.Person{
			ID:           strconv.Itoa(c.ID),
			DisplayName:  c.Name,
			PrimaryImage: profileImage(c.ProfilePath),
		})
	}
	return out
}

func crewByJob(crew []tmdbCrew, job string) []titleprovider.Person {
	out := []titleprovider.Person{}
	for _, c := range crew {
		if c.Job == job {
			out = append(out, titleprovider.Person{
				ID:           strconv.Itoa(c.ID),
				DisplayName:  c.Name,
				PrimaryImage: profileImage(c.ProfilePath),
			})
		}
	}
	return out
}

func crewByDepartment(crew []tmdbCrew, dept string) []titleprovider.Person {
	out := []titleprovider.Person{}
	seen := map[int]bool{}
	for _, c := range crew {
		if c.Department == dept && !seen[c.ID] {
			seen[c.ID] = true
			out = append(out, titleprovider.Person{
				ID:           strconv.Itoa(c.ID),
				DisplayName:  c.Name,
				PrimaryImage: profileImage(c.ProfilePath),
			})
		}
	}
	return out
}

func profileImage(path string) *titleprovider.Image {
	if path == "" {
		return nil
	}
	img := imageOf(path)
	return &img
}

func mapEpisode(e tmdbEpisode) titleprovider.Episode {
	ep := titleprovider.Episode{
		ID:            strconv.Itoa(e.ID),
		Title:         e.Name,
		PrimaryImage:  imageOf(e.StillPath),
		Season:        strconv.Itoa(e.SeasonNumber),
		EpisodeNumber: e.EpisodeNumber,
		Rating:        &titleprovider.Rating{AggregateRating: e.VoteAverage, VoteCount: e.VoteCount},
	}
	if e.Runtime != nil {
		secs := *e.Runtime * 60
		ep.RuntimeSeconds = &secs
	}
	if e.Overview != "" {
		plot := e.Overview
		ep.Plot = &plot
	}
	if rd := parseAirDate(e.AirDate); rd != nil {
		ep.ReleaseDate = rd
	}
	return ep
}

func parseAirDate(date string) *titleprovider.ReleaseDate {
	parts := strings.Split(date, "-")
	if len(parts) != 3 {
		return nil
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	return &titleprovider.ReleaseDate{Year: y, Month: m, Day: d}
}

// ----- TMDB wire types -----

type findResponse struct {
	MovieResults []findResult `json:"movie_results"`
	TVResults    []findResult `json:"tv_results"`
}
type findResult struct {
	ID int `json:"id"`
}

type movieDetails struct {
	ID                  int           `json:"id"`
	Title               string        `json:"title"`
	Overview            string        `json:"overview"`
	ReleaseDate         string        `json:"release_date"`
	Runtime             int           `json:"runtime"`
	PosterPath          string        `json:"poster_path"`
	VoteAverage         float64       `json:"vote_average"`
	VoteCount           int           `json:"vote_count"`
	Genres              []tmdbGenre   `json:"genres"`
	SpokenLanguages     []tmdbLang    `json:"spoken_languages"`
	ProductionCountries []tmdbCountry `json:"production_countries"`
	Credits             tmdbCredits   `json:"credits"`
	ExternalIDs         tmdbExternalIDs `json:"external_ids"`
}

type tvDetails struct {
	ID                  int                 `json:"id"`
	Name                string              `json:"name"`
	Overview            string              `json:"overview"`
	FirstAirDate        string              `json:"first_air_date"`
	EpisodeRunTime      []int               `json:"episode_run_time"`
	PosterPath          string              `json:"poster_path"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Genres              []tmdbGenre         `json:"genres"`
	SpokenLanguages     []tmdbLang          `json:"spoken_languages"`
	ProductionCountries []tmdbCountry       `json:"production_countries"`
	Credits             tmdbCredits         `json:"credits"`
	ExternalIDs         tmdbExternalIDs     `json:"external_ids"`
	Seasons             []tmdbSeasonSummary `json:"seasons"`
}

type tmdbSeasonSummary struct {
	SeasonNumber int `json:"season_number"`
	EpisodeCount int `json:"episode_count"`
}

type tmdbSeasonDetails struct {
	SeasonNumber int           `json:"season_number"`
	Episodes     []tmdbEpisode `json:"episodes"`
}

type tmdbEpisode struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	Runtime       *int    `json:"runtime"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

type tmdbGenre struct {
	Name string `json:"name"`
}
type tmdbLang struct {
	ISO  string `json:"iso_639_1"`
	Name string `json:"english_name"`
}
type tmdbCountry struct {
	ISO  string `json:"iso_3166_1"`
	Name string `json:"name"`
}
type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
	Crew []tmdbCrew `json:"crew"`
}
type tmdbCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}
type tmdbCrew struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}
type tmdbExternalIDs struct {
	IMDbID string `json:"imdb_id"`
}

type searchMultiResponse struct {
	Results []searchMultiResult `json:"results"`
}
type searchMultiResult struct {
	ID           int     `json:"id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	PosterPath   string  `json:"poster_path"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/titleprovider/tmdb/ -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Commit**

```bash
git add internal/titleprovider/tmdb/
git commit -m "feat(titleprovider): add TMDB provider with movie/tv/search mapping"
```

---

### Task 3: imdbapi.dev provider (port + map)

**Files:**
- Create: `internal/titleprovider/imdbapi/imdbapi.go`
- Test: `internal/titleprovider/imdbapi/imdbapi_test.go`

**Interfaces:**
- Consumes: `titleprovider` domain types (Task 1).
- Produces:
  - `imdbapi.New() *imdbapi.Provider` implementing `titleprovider.Provider`.
  - Internal test seam: `newWithBaseURL(baseURL string) *Provider`.

This ports the HTTP logic currently in `internal/imdb/imdb.go` into the provider, mapping imdbapi.dev's JSON (via local wire structs) into the domain types. `GetTitle` fetches the title, and for series also fetches seasons + all episode pages.

- [ ] **Step 1: Write the failing test**

```go
// internal/titleprovider/imdbapi/imdbapi_test.go
package imdbapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTitle_MovieMapsFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/titles/tt0068646", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"tt0068646","type":"movie","primaryTitle":"The Godfather",
			"primaryImage":{"url":"http://img/x.jpg","width":100,"height":150},
			"startYear":1972,"runtimeSeconds":10500,"genres":["Drama"],
			"rating":{"aggregateRating":8.7,"voteCount":100},"plot":"Plot.",
			"directors":[{"id":"nm1","displayName":"Coppola"}],
			"stars":[{"id":"nm2","displayName":"Brando"}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL)
	got, err := p.GetTitle(context.Background(), "tt0068646")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.Type != "movie" || got.PrimaryTitle != "The Godfather" || got.StartYear != 1972 {
		t.Fatalf("mapping wrong: %+v", got)
	}
	if got.RuntimeSeconds != 10500 || got.PrimaryImage.URL != "http://img/x.jpg" {
		t.Fatalf("mapping wrong: %+v", got)
	}
	if len(got.Directors) != 1 || got.Directors[0].DisplayName != "Coppola" {
		t.Fatalf("directors %+v", got.Directors)
	}
}

func TestGetTitle_TVFetchesSeasonsAndEpisodes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/titles/tt0903747", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"tt0903747","type":"tvSeries","primaryTitle":"Breaking Bad",
			"rating":{"aggregateRating":8.9,"voteCount":200}}`))
	})
	mux.HandleFunc("/titles/tt0903747/seasons", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seasons":[{"season":"1","episodeCount":2}]}`))
	})
	mux.HandleFunc("/titles/tt0903747/episodes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"episodes":[
			{"id":"tt10","title":"Pilot","season":"1","episodeNumber":1},
			{"id":"tt11","title":"Cat","season":"1","episodeNumber":2}],"totalCount":2}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL)
	got, err := p.GetTitle(context.Background(), "tt0903747")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.Type != "tvSeries" {
		t.Fatalf("type %q", got.Type)
	}
	if len(got.Seasons) != 1 || got.Seasons[0].Season != "1" {
		t.Fatalf("seasons %+v", got.Seasons)
	}
	if len(got.Episodes) != 2 || got.Episodes[0].Title != "Pilot" {
		t.Fatalf("episodes %+v", got.Episodes)
	}
}

func TestSearchTitles_MapsItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/titles", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"titles":[
			{"id":"tt0068646","type":"movie","primaryTitle":"The Godfather","startYear":1972,
			 "rating":{"aggregateRating":8.7,"voteCount":100}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL)
	items, err := p.SearchTitles(context.Background(), "godfather", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 1 || items[0].ID != "tt0068646" || items[0].StartYear != 1972 {
		t.Fatalf("items %+v", items)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/titleprovider/imdbapi/`
Expected: FAIL — package/functions not defined.

- [ ] **Step 3: Write the implementation**

```go
// internal/titleprovider/imdbapi/imdbapi.go
package imdbapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

const defaultBaseURL = "https://api.imdbapi.dev"

// Provider implements titleprovider.Provider against api.imdbapi.dev.
type Provider struct {
	baseURL string
	client  *http.Client
}

// New returns an imdbapi.dev provider using the public base URL.
func New() *Provider {
	return &Provider{baseURL: defaultBaseURL, client: http.DefaultClient}
}

func newWithBaseURL(baseURL string) *Provider {
	return &Provider{baseURL: baseURL, client: http.DefaultClient}
}

func (p *Provider) Name() string { return "imdbapi" }

func (p *Provider) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := p.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("imdbapi: non-2xx status %s for %s - %s", resp.Status, path, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	var wt wireTitle
	if err := p.getJSON(ctx, "/titles/"+imdbID, nil, &wt); err != nil {
		return nil, err
	}
	t := mapTitle(wt)

	if wt.Type == "tvSeries" || wt.Type == "tvMiniSeries" {
		var sr wireSeasonsResponse
		if err := p.getJSON(ctx, "/titles/"+imdbID+"/seasons", nil, &sr); err != nil {
			return nil, err
		}
		for _, s := range sr.Seasons {
			t.Seasons = append(t.Seasons, titleprovider.Season{Season: s.Season, EpisodeCount: s.EpisodeCount})
		}

		pageToken := ""
		for {
			q := url.Values{}
			q.Set("pageSize", "50")
			if pageToken != "" {
				q.Set("pageToken", pageToken)
			}
			var er wireEpisodesResponse
			if err := p.getJSON(ctx, "/titles/"+imdbID+"/episodes", q, &er); err != nil {
				return nil, err
			}
			for _, e := range er.Episodes {
				t.Episodes = append(t.Episodes, mapEpisode(e))
			}
			if er.NextPageToken == "" {
				break
			}
			pageToken = er.NextPageToken
		}
	}

	return &t, nil
}

func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))

	var sr wireSearchResponse
	if err := p.getJSON(ctx, "/search/titles", q, &sr); err != nil {
		return nil, err
	}

	items := make([]titleprovider.SearchItem, 0, len(sr.Titles))
	for _, it := range sr.Titles {
		items = append(items, titleprovider.SearchItem{
			ID:           it.ID,
			Type:         it.Type,
			PrimaryTitle: it.PrimaryTitle,
			PrimaryImage: titleprovider.Image{URL: it.PrimaryImage.URL, Width: it.PrimaryImage.Width, Height: it.PrimaryImage.Height},
			StartYear:    it.StartYear,
			Rating:       titleprovider.Rating{AggregateRating: it.Rating.AggregateRating, VoteCount: it.Rating.VoteCount},
		})
	}
	return items, nil
}

// ----- mapping -----

func mapTitle(wt wireTitle) titleprovider.Title {
	t := titleprovider.Title{
		ID:             wt.ID,
		Type:           wt.Type,
		PrimaryTitle:   wt.PrimaryTitle,
		PrimaryImage:   mapImage(wt.PrimaryImage),
		StartYear:      wt.StartYear,
		RuntimeSeconds: wt.RuntimeSeconds,
		Genres:         wt.Genres,
		Rating:         titleprovider.Rating{AggregateRating: wt.Rating.AggregateRating, VoteCount: wt.Rating.VoteCount},
		Plot:           wt.Plot,
		Directors:      mapPersons(wt.Directors),
		Writers:        mapPersons(wt.Writers),
		Stars:          mapPersons(wt.Stars),
	}
	if wt.Metacritic != nil {
		t.Metacritic = &titleprovider.Metacritic{Score: wt.Metacritic.Score, ReviewCount: wt.Metacritic.ReviewCount}
	}
	for _, c := range wt.OriginCountries {
		t.OriginCountries = append(t.OriginCountries, titleprovider.CodeName{Code: c.Code, Name: c.Name})
	}
	for _, l := range wt.SpokenLanguages {
		t.SpokenLanguages = append(t.SpokenLanguages, titleprovider.CodeName{Code: l.Code, Name: l.Name})
	}
	for _, i := range wt.Interests {
		t.Interests = append(t.Interests, titleprovider.Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre})
	}
	return t
}

func mapImage(i wireImage) titleprovider.Image {
	return titleprovider.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func mapPersons(ps []wirePerson) []titleprovider.Person {
	out := make([]titleprovider.Person, 0, len(ps))
	for _, p := range ps {
		person := titleprovider.Person{
			ID:                 p.ID,
			DisplayName:        p.DisplayName,
			AlternativeNames:   p.AlternativeNames,
			PrimaryProfessions: p.PrimaryProfessions,
		}
		if p.PrimaryImage != nil {
			img := mapImage(*p.PrimaryImage)
			person.PrimaryImage = &img
		}
		out = append(out, person)
	}
	return out
}

func mapEpisode(e wireEpisode) titleprovider.Episode {
	ep := titleprovider.Episode{
		ID:             e.ID,
		Title:          e.Title,
		PrimaryImage:   mapImage(e.PrimaryImage),
		Season:         e.Season,
		EpisodeNumber:  e.EpisodeNumber,
		RuntimeSeconds: e.RuntimeSeconds,
		Plot:           e.Plot,
	}
	if e.Rating != nil {
		ep.Rating = &titleprovider.Rating{AggregateRating: e.Rating.AggregateRating, VoteCount: e.Rating.VoteCount}
	}
	if e.ReleaseDate != nil {
		ep.ReleaseDate = &titleprovider.ReleaseDate{Year: e.ReleaseDate.Year, Month: e.ReleaseDate.Month, Day: e.ReleaseDate.Day}
	}
	return ep
}

// ----- imdbapi.dev wire types -----

type wireTitle struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	PrimaryTitle    string          `json:"primaryTitle"`
	PrimaryImage    wireImage       `json:"primaryImage"`
	StartYear       int             `json:"startYear"`
	RuntimeSeconds  int             `json:"runtimeSeconds"`
	Genres          []string        `json:"genres"`
	Rating          wireRating      `json:"rating"`
	Metacritic      *wireMetacritic `json:"metacritic,omitempty"`
	Plot            string          `json:"plot"`
	Directors       []wirePerson    `json:"directors"`
	Writers         []wirePerson    `json:"writers"`
	Stars           []wirePerson    `json:"stars"`
	OriginCountries []wireCodeName  `json:"originCountries"`
	SpokenLanguages []wireCodeName  `json:"spokenLanguages"`
	Interests       []wireInterest  `json:"interests"`
}

type wireImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type wirePerson struct {
	ID                 string     `json:"id"`
	DisplayName        string     `json:"displayName"`
	AlternativeNames   []string   `json:"alternativeNames,omitempty"`
	PrimaryImage       *wireImage `json:"primaryImage,omitempty"`
	PrimaryProfessions []string   `json:"primaryProfessions,omitempty"`
}
type wireRating struct {
	AggregateRating float64 `json:"aggregateRating"`
	VoteCount       int     `json:"voteCount"`
}
type wireMetacritic struct {
	Score       int `json:"score"`
	ReviewCount int `json:"reviewCount"`
}
type wireCodeName struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type wireInterest struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsSubgenre bool   `json:"isSubgenre,omitempty"`
}
type wireSeasonsResponse struct {
	Seasons []wireSeason `json:"seasons"`
}
type wireSeason struct {
	Season       string `json:"season"`
	EpisodeCount int    `json:"episodeCount"`
}
type wireEpisodesResponse struct {
	Episodes      []wireEpisode `json:"episodes"`
	TotalCount    int           `json:"totalCount"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}
type wireEpisode struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	PrimaryImage   wireImage        `json:"primaryImage"`
	Season         string           `json:"season"`
	EpisodeNumber  int              `json:"episodeNumber"`
	RuntimeSeconds *int             `json:"runtimeSeconds,omitempty"`
	Plot           *string          `json:"plot,omitempty"`
	Rating         *wireRating      `json:"rating,omitempty"`
	ReleaseDate    *wireReleaseDate `json:"releaseDate,omitempty"`
}
type wireReleaseDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}
type wireSearchResponse struct {
	Titles []wireSearchItem `json:"titles"`
}
type wireSearchItem struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	PrimaryTitle string     `json:"primaryTitle"`
	PrimaryImage wireImage  `json:"primaryImage"`
	StartYear    int        `json:"startYear"`
	Rating       wireRating `json:"rating"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/titleprovider/imdbapi/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/titleprovider/imdbapi/
git commit -m "feat(titleprovider): add imdbapi.dev provider behind Provider interface"
```

---

### Task 4: Provider factory (`NewFromEnv`)

> **Import-cycle note:** the factory lives in its OWN package `factory`
> (`internal/titleprovider/factory/`), NOT in `titleprovider`. The
> `tmdb`/`imdbapi` subpackages import `titleprovider` for the domain types;
> if the factory (which imports those subpackages) also lived in
> `titleprovider`, Go would reject the build with "import cycle not allowed".
> Dependency flow is one-way: `factory` → {`tmdb`, `imdbapi`} → `titleprovider`.
> Call site is therefore `factory.NewFromEnv()`, returning `titleprovider.Provider`.

**Files:**
- Create: `internal/titleprovider/factory/factory.go`
- Test: `internal/titleprovider/factory/factory_test.go`

**Interfaces:**
- Consumes: `imdbapi.New()`, `tmdb.New(apiKey)` (Tasks 2–3), `titleprovider.Provider` (Task 1).
- Produces: `factory.NewFromEnv() (titleprovider.Provider, error)`.

- [ ] **Step 1: Write the failing test**

```go
// internal/titleprovider/factory/factory_test.go
package factory_test

import (
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

func TestNewFromEnv(t *testing.T) {
	t.Run("defaults to tmdb", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "")
		t.Setenv("TMDB_API_KEY", "k")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "tmdb" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("tmdb requires key", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "tmdb")
		t.Setenv("TMDB_API_KEY", "")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error for missing TMDB_API_KEY")
		}
	})

	t.Run("imdbapi", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "imdbapi")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "imdbapi" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("unknown value errors", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "bogus")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/titleprovider/factory/ -run TestNewFromEnv`
Expected: FAIL — package/`NewFromEnv` undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/titleprovider/factory/factory.go
package factory

import (
	"fmt"
	"os"

	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/imdbapi"
	"github.com/lealre/movies-backend/internal/titleprovider/tmdb"
)

// NewFromEnv builds the provider selected by the TITLE_PROVIDER env var.
// Allowed values: "tmdb" (default), "imdbapi".
func NewFromEnv() (titleprovider.Provider, error) {
	name := os.Getenv("TITLE_PROVIDER")
	if name == "" {
		name = "tmdb"
	}

	switch name {
	case "tmdb":
		key := os.Getenv("TMDB_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=tmdb requires TMDB_API_KEY to be set")
		}
		return tmdb.New(key), nil
	case "imdbapi":
		return imdbapi.New(), nil
	default:
		return nil, fmt.Errorf("unknown TITLE_PROVIDER %q (allowed: tmdb, imdbapi)", name)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/titleprovider/factory/ -run TestNewFromEnv -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/titleprovider/factory/factory.go internal/titleprovider/factory/factory_test.go
git commit -m "feat(titleprovider): add NewFromEnv factory package (TITLE_PROVIDER)"
```

---

### Task 5: Domain → DB mapper in the titles service

**Files:**
- Create: `internal/services/titles/provider_mapper.go`
- Test: `internal/services/titles/provider_mapper_test.go`

**Interfaces:**
- Consumes: `titleprovider.Title`, `titleprovider.SearchItem` (Task 1); `mongodb.TitleDb` and nested types.
- Produces:
  - `MapProviderTitleToDb(t titleprovider.Title) mongodb.TitleDb`
  - `MapProviderSearchItemsToTitles(items []titleprovider.SearchItem) []Title`

- [ ] **Step 1: Write the failing test**

```go
// internal/services/titles/provider_mapper_test.go
package titles

import (
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

func TestMapProviderTitleToDb(t *testing.T) {
	runtime := 3540
	plot := "ep plot"
	src := titleprovider.Title{
		ID:             "tt0903747",
		Type:           "tvSeries",
		PrimaryTitle:   "Breaking Bad",
		PrimaryImage:   titleprovider.Image{URL: "u", Width: 1, Height: 2},
		StartYear:      2008,
		RuntimeSeconds: 2820,
		Genres:         []string{"Drama"},
		Rating:         titleprovider.Rating{AggregateRating: 8.9, VoteCount: 200},
		Plot:           "series plot",
		Directors:      []titleprovider.Person{{ID: "1", DisplayName: "D"}},
		OriginCountries: []titleprovider.CodeName{{Code: "US", Name: "United States"}},
		Seasons:        []titleprovider.Season{{Season: "1", EpisodeCount: 1}},
		Episodes: []titleprovider.Episode{{
			ID: "10", Title: "Pilot", Season: "1", EpisodeNumber: 1,
			RuntimeSeconds: &runtime, Plot: &plot,
			Rating:      &titleprovider.Rating{AggregateRating: 8.5, VoteCount: 50},
			ReleaseDate: &titleprovider.ReleaseDate{Year: 2008, Month: 1, Day: 20},
		}},
	}

	db := MapProviderTitleToDb(src)

	if db.ID != "tt0903747" || db.Type != "tvSeries" || db.PrimaryTitle != "Breaking Bad" {
		t.Fatalf("scalar fields wrong: %+v", db)
	}
	if db.PrimaryImage.URL != "u" || db.StartYear != 2008 || db.RuntimeSeconds != 2820 {
		t.Fatalf("fields wrong: %+v", db)
	}
	if db.Rating.AggregateRating != 8.9 || db.Rating.VoteCount != 200 {
		t.Fatalf("rating wrong: %+v", db.Rating)
	}
	if len(db.Directors) != 1 || db.Directors[0].DisplayName != "D" {
		t.Fatalf("directors wrong: %+v", db.Directors)
	}
	if len(db.OriginCountries) != 1 || db.OriginCountries[0].Name != "United States" {
		t.Fatalf("countries wrong: %+v", db.OriginCountries)
	}
	if len(db.Seasons) != 1 || db.Seasons[0].Season != "1" {
		t.Fatalf("seasons wrong: %+v", db.Seasons)
	}
	if len(db.Episodes) != 1 {
		t.Fatalf("episodes wrong: %+v", db.Episodes)
	}
	e := db.Episodes[0]
	if e.Title != "Pilot" || e.EpisodeNumber != 1 || e.RuntimeSeconds == nil || *e.RuntimeSeconds != 3540 {
		t.Fatalf("episode wrong: %+v", e)
	}
	if e.Rating == nil || e.Rating.AggregateRating != 8.5 || e.ReleaseDate == nil || e.ReleaseDate.Day != 20 {
		t.Fatalf("episode nested wrong: %+v", e)
	}
	// metacritic absent -> nil
	if db.Metacritic != nil {
		t.Fatalf("metacritic should be nil")
	}
}

func TestMapProviderSearchItemsToTitles(t *testing.T) {
	items := []titleprovider.SearchItem{{
		ID: "tt0068646", Type: "movie", PrimaryTitle: "The Godfather",
		PrimaryImage: titleprovider.Image{URL: "u"}, StartYear: 1972,
		Rating: titleprovider.Rating{AggregateRating: 8.7, VoteCount: 100},
	}}
	out := MapProviderSearchItemsToTitles(items)
	if len(out) != 1 || out[0].Id != "tt0068646" || out[0].StartYear != 1972 {
		t.Fatalf("out wrong: %+v", out)
	}
	if out[0].Rating.AggregateRating != 8.7 {
		t.Fatalf("rating wrong: %+v", out[0].Rating)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/titles/ -run TestMapProvider`
Expected: FAIL — mapper functions undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/services/titles/provider_mapper.go
package titles

import (
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// MapProviderTitleToDb converts a provider-neutral Title into the MongoDB
// document shape. This replaces the previous direct json.Unmarshal into TitleDb.
func MapProviderTitleToDb(t titleprovider.Title) mongodb.TitleDb {
	db := mongodb.TitleDb{
		ID:             t.ID,
		Type:           t.Type,
		PrimaryTitle:   t.PrimaryTitle,
		PrimaryImage:   mapProviderImage(t.PrimaryImage),
		StartYear:      t.StartYear,
		RuntimeSeconds: t.RuntimeSeconds,
		Genres:         t.Genres,
		Rating:         mongodb.Rating{AggregateRating: t.Rating.AggregateRating, VoteCount: t.Rating.VoteCount},
		Plot:           t.Plot,
		Directors:      mapProviderPersons(t.Directors),
		Writers:        mapProviderPersons(t.Writers),
		Stars:          mapProviderPersons(t.Stars),
		Seasons:        mapProviderSeasons(t.Seasons),
		Episodes:       mapProviderEpisodes(t.Episodes),
	}
	if t.Metacritic != nil {
		db.Metacritic = &mongodb.Metacritic{Score: t.Metacritic.Score, ReviewCount: t.Metacritic.ReviewCount}
	}
	for _, c := range t.OriginCountries {
		db.OriginCountries = append(db.OriginCountries, mongodb.CodeName{Code: c.Code, Name: c.Name})
	}
	for _, l := range t.SpokenLanguages {
		db.SpokenLanguages = append(db.SpokenLanguages, mongodb.CodeName{Code: l.Code, Name: l.Name})
	}
	for _, i := range t.Interests {
		db.Interests = append(db.Interests, mongodb.Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre})
	}
	return db
}

// MapProviderSearchItemsToTitles converts provider search items into the
// service-layer Title used in search responses.
func MapProviderSearchItemsToTitles(items []titleprovider.SearchItem) []Title {
	out := make([]Title, len(items))
	for i, it := range items {
		out[i] = Title{
			Id:           it.ID,
			Type:         it.Type,
			PrimaryTitle: it.PrimaryTitle,
			PrimaryImage: Image{URL: it.PrimaryImage.URL, Width: it.PrimaryImage.Width, Height: it.PrimaryImage.Height},
			StartYear:    it.StartYear,
			Rating:       Rating{AggregateRating: it.Rating.AggregateRating, VoteCount: it.Rating.VoteCount},
		}
	}
	return out
}

func mapProviderImage(i titleprovider.Image) mongodb.Image {
	return mongodb.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func mapProviderPersons(ps []titleprovider.Person) []mongodb.Person {
	out := make([]mongodb.Person, 0, len(ps))
	for _, p := range ps {
		person := mongodb.Person{
			ID:                 p.ID,
			DisplayName:        p.DisplayName,
			AlternativeNames:   p.AlternativeNames,
			PrimaryProfessions: p.PrimaryProfessions,
		}
		if p.PrimaryImage != nil {
			img := mapProviderImage(*p.PrimaryImage)
			person.PrimaryImage = &img
		}
		out = append(out, person)
	}
	return out
}

func mapProviderSeasons(ss []titleprovider.Season) []mongodb.Seasons {
	out := make([]mongodb.Seasons, len(ss))
	for i, s := range ss {
		out[i] = mongodb.Seasons{Season: s.Season, EpisodeCount: s.EpisodeCount}
	}
	return out
}

func mapProviderEpisodes(es []titleprovider.Episode) []mongodb.Episode {
	out := make([]mongodb.Episode, len(es))
	for i, e := range es {
		ep := mongodb.Episode{
			ID:             e.ID,
			Title:          e.Title,
			PrimaryImage:   mapProviderImage(e.PrimaryImage),
			Season:         e.Season,
			EpisodeNumber:  e.EpisodeNumber,
			RuntimeSeconds: e.RuntimeSeconds,
			Plot:           e.Plot,
		}
		if e.Rating != nil {
			ep.Rating = &mongodb.Rating{AggregateRating: e.Rating.AggregateRating, VoteCount: e.Rating.VoteCount}
		}
		if e.ReleaseDate != nil {
			ep.ReleaseDate = &mongodb.ReleaseDate{Year: e.ReleaseDate.Year, Month: e.ReleaseDate.Month, Day: e.ReleaseDate.Day}
		}
		out[i] = ep
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/titles/ -run TestMapProvider -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/titles/provider_mapper.go internal/services/titles/provider_mapper_test.go
git commit -m "feat(titles): add domain->TitleDb and search mappers"
```

---

### Task 6: Wire the provider into the API and migrate the titles service

This task changes several files together because they must compile as a unit: the service functions gain a `provider` parameter, the API struct carries the provider, and the handlers pass it through.

**Files:**
- Modify: `internal/api/api.go`, `internal/server/server.go`, `internal/api/titles_handlers.go:78,136`, `internal/api/groups_handler.go:244`, `internal/services/titles/titles.go`.

**Interfaces:**
- Consumes: `titleprovider.Provider` (Task 1), `MapProviderTitleToDb` / `MapProviderSearchItemsToTitles` (Task 5).
- Produces:
  - `api.NewAPI(db *mongodb.DB, provider titleprovider.Provider) *API`
  - `titles.AddNewTitle(db *mongodb.DB, provider titleprovider.Provider, ctx context.Context, titleId string) (Title, error)`
  - `titles.SearchTitles(provider titleprovider.Provider, ctx context.Context, searchQuery string, limit int) ([]Title, error)`

- [ ] **Step 1: Update the API struct** (`internal/api/api.go`)

```go
package api

import (
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

type ErrorResponse struct {
	StatusCode   int    `json:"statusCode"`
	ErrorMessage string `json:"errorMessage"`
}

type DefaultResponse struct {
	Message string `json:"message"`
}

type API struct {
	Db       *mongodb.DB
	Secret   *string
	Provider titleprovider.Provider
}

func NewAPI(db *mongodb.DB, provider titleprovider.Provider) *API {
	return &API{Db: db, Provider: provider}
}

var PublicPaths = map[string]bool{
	"POST /login": true,
	"POST /users": true,
}
```

- [ ] **Step 2: Update the server wiring** (`internal/server/server.go`)

Add the import and build the provider in `NewServer`, returning an error if the provider cannot be built. Change the `NewServer` and `ListenAndServe` signatures to propagate the error.

Replace the top of `NewServer` (lines 13–17) and the import block:

```go
import (
	"fmt"
	"log"
	"net/http"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
	"go.mongodb.org/mongo-driver/mongo"
)

func NewServer(db *mongo.Client) (http.Handler, error) {
	mux := http.NewServeMux()

	dbClient := mongodb.NewDB(db)

	provider, err := factory.NewFromEnv()
	if err != nil {
		return nil, err
	}
	log.Printf("Using title provider: %s", provider.Name())

	a := api.NewAPI(dbClient, provider)
	// ... (rest of NewServer unchanged, but the final return becomes:)
	//   return handler, nil
}
```

Update the final two lines of `NewServer` from `return handler` to `return handler, nil`.

Update `ListenAndServe`:

```go
func ListenAndServe(db *mongo.Client) error {
	handler, err := NewServer(db)
	if err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	log.Println("Server running on :8080")
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error while starting server: %v", err)
	}
	log.Println("Server started listening on port 8080")
	return nil
}
```

(`main.go` calls `server.ListenAndServe(db)` and already handles a returned error — no change needed.)

- [ ] **Step 3: Migrate the titles service** (`internal/services/titles/titles.go`)

Update the import block: remove `"encoding/json"` if it becomes unused (it is still used elsewhere? check — after this change `json` is no longer used in this file; remove it) and replace `"github.com/lealre/movies-backend/internal/imdb"` with `"github.com/lealre/movies-backend/internal/titleprovider"`.

Replace `AddNewTitle` (lines 150–234) with:

```go
func AddNewTitle(db *mongodb.DB, provider titleprovider.Provider, ctx context.Context, titleId string) (Title, error) {
	logger := logx.FromContext(ctx)

	providerTitle, err := provider.GetTitle(ctx, titleId)
	if err != nil {
		return Title{}, err
	}
	if providerTitle.Type == "tvSeries" || providerTitle.Type == "tvMiniSeries" {
		logger.Printf("Title %s is a TV series with %d seasons", titleId, len(providerTitle.Seasons))
	}

	title := MapProviderTitleToDb(*providerTitle)

	// Set missing fields
	now := time.Now()
	title.AddedAt = &now
	title.UpdatedAt = &now

	doc, err := bson.Marshal(title)
	if err != nil {
		return Title{}, err
	}

	var bsonDoc bson.M
	if err := bson.Unmarshal(doc, &bsonDoc); err != nil {
		return Title{}, err
	}

	if err := db.AddTitle(ctx, bsonDoc); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return Title{}, err
		}
		// If duplicate, read back the stored document
		if stored, gerr := db.GetTitleById(ctx, titleId); gerr == nil {
			title = stored
		}
	}

	return MapDbTitleToApiTitle(title), nil
}
```

Note: the previous code passed `bson.M` to `db.AddTitle` (signature `AddTitle(ctx, doc map[string]any)`), so `bsonDoc` stays. The duplicate-key read-back now uses `db.GetTitleById` (returns `mongodb.TitleDb`) instead of the previous broken `TitleExists` round-trip.

Replace `SearchTitles` (lines 254–266) with:

```go
func SearchTitles(provider titleprovider.Provider, ctx context.Context, searchQuery string, limit int) ([]Title, error) {
	items, err := provider.SearchTitles(ctx, searchQuery, limit)
	if err != nil {
		return nil, err
	}
	return MapProviderSearchItemsToTitles(items), nil
}
```

- [ ] **Step 4: Update the handlers**

`internal/api/titles_handlers.go` line 78:

```go
	title, err := titles.AddNewTitle(api.Db, api.Provider, r.Context(), titleID)
```

`internal/api/titles_handlers.go` line 136:

```go
	titles, err := titles.SearchTitles(api.Provider, r.Context(), searchQuery, limit)
```

`internal/api/groups_handler.go` line 244:

```go
		_, err = titles.AddNewTitle(api.Db, api.Provider, r.Context(), titleID)
```

- [ ] **Step 5: Build the affected packages**

Run: `go build ./internal/... ./...`
Expected: builds successfully. (If `internal/services/titles/mapper.go` still imports `imdb`, it will fail here — that is fixed in Task 7, which must land together; if you hit the error now, proceed to Task 7 before committing.)

Because `mapper.go` and `cmd/routines` still reference `internal/imdb`, the whole-module build only goes green after Task 7. To keep this task independently testable, build just the wired packages:

Run: `go build ./internal/api/... ./internal/server/... ./internal/services/titles/...`
Expected: PASS (these no longer reference `imdb`; `mapper.go` does — so also do Task 7 Step 1 before this passes). See note below.

> Ordering note: Tasks 6 and 7 both touch `internal/services/titles`. Land them in one review unit — do Task 7 Step 1 (retype `mapper.go`) immediately, then build. Commit both together in Task 7 Step 5.

- [ ] **Step 6: Commit** — deferred; committed with Task 7 (they share the titles package).

---

### Task 7: Retype the legacy mapper helpers + migrate the cron routine

**Files:**
- Modify: `internal/services/titles/mapper.go`, `cmd/routines/main.go`.

**Interfaces:**
- Consumes: `titleprovider` domain types, `provider.GetTitle` (Tasks 1–4), `MapProviderTitleToDb` unused here.
- Produces: `MapImdbSeasonsToDbSeasons` / `MapImdbEpisodesToDbEpisodes` retyped to accept `[]titleprovider.Season` / `[]titleprovider.Episode` (still called by routines).

- [ ] **Step 1: Retype `mapper.go`**

In `internal/services/titles/mapper.go`, change the import `"github.com/lealre/movies-backend/internal/imdb"` to `"github.com/lealre/movies-backend/internal/titleprovider"`, and update the three functions that reference `imdb.*`:

```go
func MapImdbSeasonsToDbSeasons(seasons []titleprovider.Season) []mongodb.Seasons {
	dbSeasons := make([]mongodb.Seasons, len(seasons))
	for i, season := range seasons {
		dbSeasons[i] = mongodb.Seasons{
			Season:       season.Season,
			EpisodeCount: season.EpisodeCount,
		}
	}
	return dbSeasons
}

func MapImdbEpisodesToDbEpisodes(episodes []titleprovider.Episode) []mongodb.Episode {
	dbEpisodes := make([]mongodb.Episode, len(episodes))
	for i, episode := range episodes {
		dbEpisode := mongodb.Episode{
			ID:    episode.ID,
			Title: episode.Title,
			PrimaryImage: mongodb.Image{
				URL:    episode.PrimaryImage.URL,
				Width:  episode.PrimaryImage.Width,
				Height: episode.PrimaryImage.Height,
			},
			Season:        episode.Season,
			EpisodeNumber: episode.EpisodeNumber,
		}
		if episode.RuntimeSeconds != nil {
			dbEpisode.RuntimeSeconds = episode.RuntimeSeconds
		}
		if episode.Plot != nil {
			dbEpisode.Plot = episode.Plot
		}
		if episode.Rating != nil {
			dbEpisode.Rating = &mongodb.Rating{
				AggregateRating: episode.Rating.AggregateRating,
				VoteCount:       episode.Rating.VoteCount,
			}
		}
		if episode.ReleaseDate != nil {
			dbEpisode.ReleaseDate = &mongodb.ReleaseDate{
				Year:  episode.ReleaseDate.Year,
				Month: episode.ReleaseDate.Month,
				Day:   episode.ReleaseDate.Day,
			}
		}
		dbEpisodes[i] = dbEpisode
	}
	return dbEpisodes
}

// MapImdbSearchTitlesToTitles is removed — superseded by MapProviderSearchItemsToTitles.
```

Delete `MapImdbSearchTitlesToTitles` (it referenced `imdb.SearchTitleItem` and is no longer used — `SearchTitles` now uses `MapProviderSearchItemsToTitles`). Keep `MapDbTitleToApiTitle`, `MapDbSeasonsToImdbSeasons`, and `MapDbEpisodesToImdbEpisodes` unchanged (they use `mongodb.*` and service-layer types only).

- [ ] **Step 2: Migrate the cron routine** (`cmd/routines/main.go`)

The routine no longer batches; it fetches each title fully via `provider.GetTitle`. Replace the whole file with:

```go
package main

import (
	"context"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	_ = godotenv.Load()

	log.Println("")
	log.Println("==========================================")
	log.Println("🎬 Starting titles update...")
	log.Println("==========================================")

	provider, err := factory.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to build title provider: %v", err)
	}
	log.Printf("Using title provider: %s", provider.Name())

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	collection := db.Collection(mongodb.TitlesCollection)

	log.Println("Fetching all title IDs from database...")
	titleIDs, err := getAllTitleIDs(ctx, collection)
	if err != nil {
		log.Fatalf("Failed to fetch title IDs: %v", err)
	}
	log.Printf("Found %d titles to sync", len(titleIDs))

	if err := syncTitles(ctx, provider, collection, titleIDs); err != nil {
		log.Fatalf("Failed to sync titles: %v", err)
	}
	log.Println("Sync completed successfully")
}

func getAllTitleIDs(ctx context.Context, collection *mongo.Collection) ([]string, error) {
	opts := options.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var titleIDs []string
	for cursor.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		titleIDs = append(titleIDs, doc.ID)
	}
	return titleIDs, cursor.Err()
}

func syncTitles(ctx context.Context, provider titleprovider.Provider, collection *mongo.Collection, titleIDs []string) error {
	jobs := make(chan string, len(titleIDs))
	wg := sync.WaitGroup{}
	workerCount := 5

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for titleID := range jobs {
				if err := processTitle(ctx, provider, collection, titleID); err != nil {
					log.Printf("failed processing %s: %v", titleID, err)
				}
			}
		}()
	}

	for _, id := range titleIDs {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	return nil
}

func processTitle(ctx context.Context, provider titleprovider.Provider, collection *mongo.Collection, titleID string) error {
	var dbTitle struct {
		ID           string              `bson:"_id"`
		Type         string              `bson:"type"`
		PrimaryImage mongodb.Image       `bson:"primaryImage"`
		Seasons      []mongodb.Seasons   `bson:"seasons"`
		Episodes     []mongodb.Episode   `bson:"episodes"`
		Rating       mongodb.Rating      `bson:"rating"`
		Metacritic   *mongodb.Metacritic `bson:"metacritic,omitempty"`
	}

	projection := bson.M{
		"_id": 1, "type": 1, "primaryImage": 1, "seasons": 1,
		"episodes": 1, "rating": 1, "metacritic": 1,
	}

	err := collection.FindOne(ctx, bson.M{"_id": titleID}, options.FindOne().SetProjection(projection)).Decode(&dbTitle)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Title %s not found in database, skipping", titleID)
			return nil
		}
		return err
	}

	apiTitle, err := provider.GetTitle(ctx, titleID)
	if err != nil {
		return err
	}

	updateDoc := bson.M{}
	hasChanges := false

	apiPrimaryImage := mongodb.Image{
		URL:    apiTitle.PrimaryImage.URL,
		Width:  apiTitle.PrimaryImage.Width,
		Height: apiTitle.PrimaryImage.Height,
	}
	if !imagesEqual(dbTitle.PrimaryImage, apiPrimaryImage) {
		updateDoc["primaryImage"] = apiPrimaryImage
		hasChanges = true
	}

	apiSeasons := titles.MapImdbSeasonsToDbSeasons(apiTitle.Seasons)
	if !reflect.DeepEqual(dbTitle.Seasons, apiSeasons) {
		updateDoc["seasons"] = apiSeasons
		hasChanges = true
	}

	apiEpisodes := titles.MapImdbEpisodesToDbEpisodes(apiTitle.Episodes)
	if !reflect.DeepEqual(dbTitle.Episodes, apiEpisodes) {
		updateDoc["episodes"] = apiEpisodes
		hasChanges = true
	}

	apiRating := mongodb.Rating{
		AggregateRating: apiTitle.Rating.AggregateRating,
		VoteCount:       apiTitle.Rating.VoteCount,
	}
	if dbTitle.Rating != apiRating {
		updateDoc["rating"] = apiRating
		hasChanges = true
	}

	var apiMetacritic *mongodb.Metacritic
	if apiTitle.Metacritic != nil {
		apiMetacritic = &mongodb.Metacritic{Score: apiTitle.Metacritic.Score, ReviewCount: apiTitle.Metacritic.ReviewCount}
	}
	if !metacriticEqual(dbTitle.Metacritic, apiMetacritic) {
		updateDoc["metacritic"] = apiMetacritic
		hasChanges = true
	}

	updateDoc["updatedAt"] = time.Now()

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": titleID}, bson.M{"$set": updateDoc}); err != nil {
		return err
	}

	if hasChanges {
		log.Printf("Updated title %s (fields changed)", titleID)
	} else {
		log.Printf("Updated title %s (updatedAt only)", titleID)
	}
	return nil
}

func imagesEqual(a, b mongodb.Image) bool {
	return a.URL == b.URL && a.Width == b.Width && a.Height == b.Height
}

func metacriticEqual(a, b *mongodb.Metacritic) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Score == b.Score && a.ReviewCount == b.ReviewCount
}
```

- [ ] **Step 3: Build the module (routines + services + api)**

Run: `go build ./internal/... ./cmd/routines/...`
Expected: PASS (`internal/imdb` is still present and used only by `cmd/test-fixtures` now).

- [ ] **Step 4: Run the unit tests for touched packages**

Run: `go test ./internal/titleprovider/... ./internal/services/titles/...`
Expected: PASS

- [ ] **Step 5: Commit (Tasks 6 + 7 together)**

```bash
git add internal/api/api.go internal/server/server.go internal/api/titles_handlers.go \
  internal/api/groups_handler.go internal/services/titles/titles.go \
  internal/services/titles/mapper.go cmd/routines/main.go
git commit -m "feat: inject title provider into API and cron routine"
```

---

### Task 8: Hermetic test provider + migrate test-fixtures & `seedTitles`

> **Why this differs from the original spec:** the spec wrongly assumed no
> integration test calls the provider. In fact `tests/titles_test.go`'s
> "add title as admin" test does `POST /titles` → `AddNewTitle` →
> `provider.GetTitle`, and after Task 6 `NewServer` also builds the provider
> from env (erroring without `TMDB_API_KEY`). Decision (approved): make the
> integration suite hermetic by injecting a fixture-backed **fake provider**
> via a new `NewServerWithProvider` seam — no network, no API key in CID.

**Files:**
- Modify: `internal/server/server.go` (add `NewServerWithProvider` seam), `tests/setup_test.go` (inject fake provider), `tests/titles_setup_test.go` (`seedTitles`/loaders → `mongodb.TitleDb`), `cmd/test-fixtures/main.go`.
- Create: `tests/fake_provider_test.go` (fixture-backed fake provider).

**Interfaces:**
- Consumes: `titleprovider.Provider` (Task 1), `factory.NewFromEnv` (Task 4), `titles.MapProviderTitleToDb` (Task 5), `mongodb.TitleDb`.
- Produces:
  - `server.NewServerWithProvider(db *mongo.Client, provider titleprovider.Provider) http.Handler`
  - `server.NewServer(db *mongo.Client) (http.Handler, error)` now builds the provider via `factory.NewFromEnv()` and delegates to `NewServerWithProvider`.
  - test helper `newFakeTitleProvider() *fakeTitleProvider` implementing `titleprovider.Provider`.
  - `seedTitles(t, []mongodb.TitleDb)`, loaders returning `[]mongodb.TitleDb`.

> Do NOT run `cmd/test-fixtures` as part of this change — it would overwrite the committed fixtures with TMDB-shaped data and break existing assertions. It only needs to compile.

- [ ] **Step 1: Add the `NewServerWithProvider` seam** (`internal/server/server.go`)

Refactor so the route-wiring body lives in `NewServerWithProvider`, and `NewServer` builds the provider then delegates. Replace the current `NewServer` function (the whole `func NewServer(db *mongo.Client) (http.Handler, error) { ... }` from Task 6) with these TWO functions. **Move the existing route-registration body verbatim** (all the `mux.HandleFunc(...)` lines and the middleware wrapping) into `NewServerWithProvider` — do not retype or reorder the routes.

```go
// NewServer builds the production server, selecting the title provider from env.
func NewServer(db *mongo.Client) (http.Handler, error) {
	provider, err := factory.NewFromEnv()
	if err != nil {
		return nil, err
	}
	log.Printf("Using title provider: %s", provider.Name())
	return NewServerWithProvider(db, provider), nil
}

// NewServerWithProvider builds the server with an explicit title provider.
// Tests use this to inject a fixture-backed fake provider (no network).
func NewServerWithProvider(db *mongo.Client, provider titleprovider.Provider) http.Handler {
	mux := http.NewServeMux()

	dbClient := mongodb.NewDB(db)
	a := api.NewAPI(dbClient, provider)

	// TODO: Updated this
	secret := "my-secret"
	a.Secret = &secret

	// ... MOVE ALL EXISTING mux.HandleFunc(...) ROUTE REGISTRATIONS HERE VERBATIM ...

	handler := AuthMiddleware(*a.Secret, dbClient)(mux)
	handler = RequestIdMiddleware(handler) // wrap LAST → runs FIRST

	return handler
}
```

Imports: `NewServerWithProvider` now references `titleprovider.Provider`, so the import block needs BOTH `"github.com/lealre/movies-backend/internal/titleprovider"` and `"github.com/lealre/movies-backend/internal/titleprovider/factory"`.

- [ ] **Step 2: Create the fake provider** (`tests/fake_provider_test.go`)

```go
package tests

import (
	"context"
	"encoding/json"
	"os"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// fakeTitleProvider implements titleprovider.Provider for integration tests,
// backed by the on-disk fixtures. It performs no network calls, so the suite
// needs neither TMDB_API_KEY nor connectivity.
type fakeTitleProvider struct {
	byID map[string]mongodb.TitleDb
}

func newFakeTitleProvider() *fakeTitleProvider {
	f := &fakeTitleProvider{byID: map[string]mongodb.TitleDb{}}
	for _, path := range []string{MOVIE_TILES_FIXTURES_PATH, TV_SERIES_TILES_FIXTURES_PATH} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // fixtures optional; GetTitle returns ErrTitleNotFound if absent
		}
		var docs []mongodb.TitleDb
		if err := json.Unmarshal(data, &docs); err != nil {
			continue
		}
		for _, d := range docs {
			f.byID[d.ID] = d
		}
	}
	return f
}

func (f *fakeTitleProvider) Name() string { return "fake" }

func (f *fakeTitleProvider) GetTitle(_ context.Context, imdbID string) (*titleprovider.Title, error) {
	d, ok := f.byID[imdbID]
	if !ok {
		return nil, titleprovider.ErrTitleNotFound
	}
	t := titleprovider.Title{
		ID:             d.ID,
		Type:           d.Type,
		PrimaryTitle:   d.PrimaryTitle,
		PrimaryImage:   titleprovider.Image{URL: d.PrimaryImage.URL, Width: d.PrimaryImage.Width, Height: d.PrimaryImage.Height},
		StartYear:      d.StartYear,
		RuntimeSeconds: d.RuntimeSeconds,
		Genres:         d.Genres,
		Rating:         titleprovider.Rating{AggregateRating: d.Rating.AggregateRating, VoteCount: d.Rating.VoteCount},
		Plot:           d.Plot,
	}
	return &t, nil
}

func (f *fakeTitleProvider) SearchTitles(_ context.Context, _ string, _ int) ([]titleprovider.SearchItem, error) {
	return nil, nil
}
```

- [ ] **Step 3: Inject the fake in test setup** (`tests/setup_test.go`)

The current line `handler := server.NewServer(testClient)` breaks (Task 6 made `NewServer` return two values). Replace it with the injected fake provider:

```go
	handler := server.NewServerWithProvider(testClient, newFakeTitleProvider())
```

(No error to handle — `NewServerWithProvider` returns only `http.Handler`.)

- [ ] **Step 4: Migrate `seedTitles` + loaders** (`tests/titles_setup_test.go`)

Replace the `internal/imdb` import with `internal/mongodb` (already imported — so just drop the `imdb` import) and change every `[]imdb.Title` to `[]mongodb.TitleDb`:
- `func seedTitles(t *testing.T, titles []mongodb.TitleDb)`
- `loadTitlesFixture` returns `[]mongodb.TitleDb`; its local `var docs []imdb.Title` → `var docs []mongodb.TitleDb`
- `loadTVSeriesTitlesFixture` returns `[]mongodb.TitleDb`; same local change

The existing fixture files unmarshal cleanly into `mongodb.TitleDb` (identical json tags). Tests only read `.ID`/`.PrimaryTitle`/`.Type` off the results (verified), so no call-site changes are needed. After editing, run `grep -rn "imdb\." tests/` and fix any stragglers (there should be none).

- [ ] **Step 5: Rewrite `cmd/test-fixtures/main.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

func main() {
	_ = godotenv.Load()

	provider, err := factory.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to build title provider: %v", err)
	}
	log.Printf("Using title provider: %s", provider.Name())

	movieTitles := []string{"tt0068646", "tt0075148", "tt1092016", "tt0381707", "tt0133093"}
	tvSeriesTitles := []string{"tt1190634", "tt0903747"}

	ctx := context.Background()

	movieTitlesToExport := make([]mongodb.TitleDb, len(movieTitles))
	for i, titleID := range movieTitles {
		log.Printf("Fetching movie title: %s", titleID)
		t, err := provider.GetTitle(ctx, titleID)
		if err != nil {
			log.Fatalf("Error fetching movie title %s: %v", titleID, err)
		}
		movieTitlesToExport[i] = titles.MapProviderTitleToDb(*t)
	}

	tvSeriesTitlesToExport := make([]mongodb.TitleDb, len(tvSeriesTitles))
	for i, titleID := range tvSeriesTitles {
		log.Printf("Fetching TV series title: %s", titleID)
		t, err := provider.GetTitle(ctx, titleID)
		if err != nil {
			log.Fatalf("Error fetching TV series title %s: %v", titleID, err)
		}
		tvSeriesTitlesToExport[i] = titles.MapProviderTitleToDb(*t)
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	moviePath := filepath.Join(rootDir, "tests/fixtures/movieTitles.json")
	if err := writeFixture(moviePath, movieTitlesToExport); err != nil {
		log.Fatalf("Error writing movie titles fixture: %v", err)
	}
	log.Printf("Successfully created movie titles fixture: %s", moviePath)

	tvSeriesPath := filepath.Join(rootDir, "tests/fixtures/tvSeriesTitles.json")
	if err := writeFixture(tvSeriesPath, tvSeriesTitlesToExport); err != nil {
		log.Fatalf("Error writing TV series titles fixture: %v", err)
	}
	log.Printf("Successfully created TV series titles fixture: %s", tvSeriesPath)
}

func writeFixture(filePath string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonData, 0644)
}
```

- [ ] **Step 2: Update `seedTitles`** (`tests/titles_setup_test.go`)

Change the import `"github.com/lealre/movies-backend/internal/imdb"` to `"github.com/lealre/movies-backend/internal/mongodb"` (if not already imported) and change the `seedTitles` signature and fixture unmarshal target from `[]imdb.Title` to `[]mongodb.TitleDb`. The existing fixtures on disk are JSON-compatible with `mongodb.TitleDb` (identical field tags), so no fixture regeneration is needed.

Read the file first, then update line 19 (`func seedTitles(t *testing.T, titles []imdb.Title) {`) to:

```go
func seedTitles(t *testing.T, titles []mongodb.TitleDb) {
```

and update the two fixture loader functions (around lines 40–75) that unmarshal into `[]imdb.Title` to unmarshal into `[]mongodb.TitleDb`. Also update any test call sites that build `[]imdb.Title` literals to `[]mongodb.TitleDb` (search: `grep -rn "imdb\." tests/`).

- [ ] **Step 6: Build everything + vet**

Run: `go build ./... && go vet ./...`
Expected: PASS (whole module compiles, incl. `tests/` and `cmd/test-fixtures`; `internal/imdb` still exists, removed in Task 9).

- [ ] **Step 7: Run the full integration suite (now hermetic)**

Run: `go test ./tests/ -count=1`
Expected: PASS — the fake provider serves `POST /titles` from fixtures, so no `TMDB_API_KEY` or network is needed. (Requires Docker for the MongoDB testcontainer. If Docker is unavailable in this environment, instead run `go test ./tests/ -run TestNothing -count=1` to prove the package compiles, and report that the full run could not execute for lack of Docker — do NOT report the suite as passing if it did not run.)

- [ ] **Step 8: Commit**

```bash
git add internal/server/server.go tests/fake_provider_test.go tests/setup_test.go \
  tests/titles_setup_test.go cmd/test-fixtures/main.go
git commit -m "test: hermetic fake title provider; migrate fixtures off internal/imdb"
```

---

### Task 9: Remove `internal/imdb`, update config & changelog, full verification

**Files:**
- Remove: `internal/imdb/imdb.go`, `internal/imdb/types.go`.
- Modify: `env.example`, `CHANGELOG.md`.

- [ ] **Step 1: Confirm nothing references `internal/imdb`**

Run: `grep -rn "movies-backend/internal/imdb" . --include=*.go`
Expected: no output.

- [ ] **Step 2: Remove the package**

```bash
git rm internal/imdb/imdb.go internal/imdb/types.go
```

- [ ] **Step 3: Update `env.example`**

Add these lines (append near the other config):

```bash
# Title metadata provider: tmdb (default) or imdbapi
TITLE_PROVIDER=tmdb
# TMDB v3 API key (required when TITLE_PROVIDER=tmdb)
TMDB_API_KEY=
```

- [ ] **Step 4: Update `CHANGELOG.md`**

Add a new section at the top:

```markdown
<a name="v0.0.9"></a>
## [v0.0.9](https://github.com/lealre/fs-mcp/compare/v0.0.8...v0.0.9) (2026-07-19)

* Add pluggable title metadata provider behind a Provider interface
* Add TMDB provider and select provider via TITLE_PROVIDER env var
* Migrate off api.imdbapi.dev (service discontinued) to TMDB
* Keep imdbapi.dev provider for env-var port-back
```

- [ ] **Step 5: Full build + vet + unit tests**

Run: `go build ./... && go vet ./...`
Expected: PASS

Run: `go test ./internal/...`
Expected: PASS

- [ ] **Step 6: Full integration test suite** (requires Docker for testcontainers)

Run: `go test ./tests/ -count=1`
Expected: PASS (unchanged behavior — fixtures untouched).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "chore: remove internal/imdb, add TITLE_PROVIDER config, changelog v0.0.9"
```

---

## Post-implementation (manual, outside the plan)

- Verify end-to-end with the `verify` skill: with `TITLE_PROVIDER=tmdb` and a real `TMDB_API_KEY`, exercise `POST /titles` (add by IMDb URL) and `GET /titles/search?query=...` against a running server, confirming a movie and a TV series ingest with seasons/episodes.
- On merge to `main`, tag `v0.0.9` on the merge commit (lightweight, matching convention) and push tags.
- Mark the v0.0.9 checkboxes complete in the Notion page.

## Self-Review Notes

- **Spec coverage:** package structure (Tasks 1–4), Provider interface (Task 1), neutral domain types (Task 1), TMDB internals incl. resolve-on-search + skip Specials + image URLs + metacritic/interests empty (Task 2), imdbapi provider (Task 3), factory + default tmdb + unknown error (Task 4), domain→DB mapping with DB shape unchanged (Task 5), DI wiring (Task 6), cron refresh via GetTitle (Task 7), test-fixtures + seedTitles + fixtures-not-regenerated (Task 8), remove imdb + env.example + changelog + v0.0.9 tag note (Task 9). All spec sections mapped.
- **Type consistency:** `AddNewTitle(db, provider, ctx, titleId)` and `SearchTitles(provider, ctx, query, limit)` signatures are used identically in Task 6 handlers. `MapImdbSeasonsToDbSeasons([]titleprovider.Season)` / `MapImdbEpisodesToDbEpisodes([]titleprovider.Episode)` retyped in Task 7 and called with `apiTitle.Seasons` / `apiTitle.Episodes` (both `[]titleprovider.*`). `MapProviderTitleToDb` / `MapProviderSearchItemsToTitles` defined in Task 5, used in Tasks 6 & 8.
- **Ordering:** Tasks 6 and 7 share the `titles` package and are committed together (noted in both).
