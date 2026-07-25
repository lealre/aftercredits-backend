package omdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

func newTestServer(routes map[string]string) *httptest.Server {
	mux := http.NewServeMux()
	// OMDb serves everything from "/" with query params, so route on the raw query.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		key := q.Get("i") + "|" + q.Get("Season") + "|" + q.Get("s")
		body, ok := routes[key]
		if !ok {
			http.Error(w, `{"Response":"False","Error":"not routed"}`, http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

func TestGetTitle_Movie(t *testing.T) {
	srv := newTestServer(map[string]string{
		"tt11378946||": `{"Title":"Michael","Year":"2026","Rated":"PG-13","Released":"24 Apr 2026",
			"Runtime":"127 min","Genre":"Drama, Music","Director":"Antoine Fuqua","Writer":"John Logan",
			"Actors":"Jaafar Jackson, Nia Long, Miles Teller","Plot":"A biopic.","Language":"English",
			"Country":"United States","Poster":"https://img/michael.jpg","Metascore":"55",
			"imdbRating":"7.4","imdbVotes":"157,000","imdbID":"tt11378946","Type":"movie","Response":"True"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	got, err := p.GetTitle(context.Background(), "tt11378946")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.ID != "tt11378946" || got.Type != "movie" || got.PrimaryTitle != "Michael" {
		t.Fatalf("scalar fields wrong: %+v", got)
	}
	if got.StartYear != 2026 {
		t.Fatalf("year %d", got.StartYear)
	}
	if got.RuntimeSeconds != 127*60 {
		t.Fatalf("runtime %d", got.RuntimeSeconds)
	}
	if len(got.Genres) != 2 || got.Genres[0] != "Drama" || got.Genres[1] != "Music" {
		t.Fatalf("genres %v", got.Genres)
	}
	if got.Rating.AggregateRating != 7.4 || got.Rating.VoteCount != 157000 {
		t.Fatalf("rating %+v (want 7.4/157000)", got.Rating)
	}
	if got.Metacritic == nil || got.Metacritic.Score != 55 {
		t.Fatalf("metacritic %+v (want 55)", got.Metacritic)
	}
	if got.PrimaryImage.URL != "https://img/michael.jpg" {
		t.Fatalf("image %q", got.PrimaryImage.URL)
	}
	if len(got.Directors) != 1 || got.Directors[0].DisplayName != "Antoine Fuqua" {
		t.Fatalf("directors %+v", got.Directors)
	}
	if len(got.Stars) != 3 || got.Stars[0].DisplayName != "Jaafar Jackson" {
		t.Fatalf("stars %+v", got.Stars)
	}
}

func TestGetTitle_SeriesWithSeasons(t *testing.T) {
	srv := newTestServer(map[string]string{
		"tt0903747||": `{"Title":"Breaking Bad","Year":"2008–2013","Runtime":"49 min",
			"Genre":"Crime, Drama","Director":"N/A","Writer":"Vince Gilligan","Actors":"Bryan Cranston, Aaron Paul",
			"Plot":"A teacher.","Poster":"https://img/bb.jpg","Metascore":"N/A","imdbRating":"9.5",
			"imdbVotes":"2,000,000","imdbID":"tt0903747","Type":"series","totalSeasons":"2","Response":"True"}`,
		"tt0903747|1|": `{"Title":"Breaking Bad","Season":"1","totalSeasons":"2","Episodes":[
			{"Title":"Pilot","Released":"2008-01-20","Episode":"1","imdbRating":"8.9","imdbID":"tt0959621"},
			{"Title":"Cat","Released":"2008-01-27","Episode":"2","imdbRating":"8.6","imdbID":"tt1054724"}],"Response":"True"}`,
		"tt0903747|2|": `{"Title":"Breaking Bad","Season":"2","totalSeasons":"2","Episodes":[
			{"Title":"Seven Thirty-Seven","Released":"2009-03-08","Episode":"1","imdbRating":"8.7","imdbID":"tt1232244"}],"Response":"True"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	got, err := p.GetTitle(context.Background(), "tt0903747")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	if got.Type != "tvSeries" || got.StartYear != 2008 {
		t.Fatalf("type/year: %q %d", got.Type, got.StartYear)
	}
	if got.Metacritic != nil {
		t.Fatalf("metacritic should be nil when N/A")
	}
	if len(got.Seasons) != 2 || got.Seasons[0].Season != "1" || got.Seasons[0].EpisodeCount != 2 {
		t.Fatalf("seasons %+v", got.Seasons)
	}
	if len(got.Episodes) != 3 {
		t.Fatalf("episodes %d (want 3)", len(got.Episodes))
	}
	e := got.Episodes[0]
	if e.ID != "tt0959621" || e.Season != "1" || e.EpisodeNumber != 1 || e.Title != "Pilot" {
		t.Fatalf("ep0 %+v", e)
	}
	if e.Rating == nil || e.Rating.AggregateRating != 8.9 {
		t.Fatalf("ep0 rating %+v", e.Rating)
	}
	if e.ReleaseDate == nil || e.ReleaseDate.Year != 2008 || e.ReleaseDate.Month != 1 || e.ReleaseDate.Day != 20 {
		t.Fatalf("ep0 releaseDate %+v", e.ReleaseDate)
	}
}

func TestGetTitle_NotFound(t *testing.T) {
	srv := newTestServer(map[string]string{
		"tt0000000||": `{"Response":"False","Error":"Incorrect IMDb ID."}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	_, err := p.GetTitle(context.Background(), "tt0000000")
	if !errors.Is(err, titleprovider.ErrTitleNotFound) {
		t.Fatalf("expected ErrTitleNotFound, got %v", err)
	}
}

func TestGetTitle_RequestLimitIsNotNotFound(t *testing.T) {
	srv := newTestServer(map[string]string{
		"tt0068646||": `{"Response":"False","Error":"Request limit reached!"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	_, err := p.GetTitle(context.Background(), "tt0068646")
	if err == nil || errors.Is(err, titleprovider.ErrTitleNotFound) {
		t.Fatalf("limit error must be a real error, not ErrTitleNotFound; got %v", err)
	}
}

func TestSearchTitles(t *testing.T) {
	srv := newTestServer(map[string]string{
		"||godfather": `{"Search":[
			{"Title":"The Godfather","Year":"1972","imdbID":"tt0068646","Type":"movie","Poster":"https://img/gf.jpg"},
			{"Title":"The Godfather Part II","Year":"1974","imdbID":"tt0071562","Type":"movie","Poster":"N/A"}],
			"totalResults":"2","Response":"True"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	items, err := p.SearchTitles(context.Background(), "godfather", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].ID != "tt0068646" || items[0].Type != "movie" || items[0].StartYear != 1972 {
		t.Fatalf("item0 %+v", items[0])
	}
	if items[0].PrimaryImage.URL != "https://img/gf.jpg" {
		t.Fatalf("item0 image %q", items[0].PrimaryImage.URL)
	}
	if items[1].PrimaryImage.URL != "" { // "N/A" -> empty
		t.Fatalf("item1 image should be empty for N/A, got %q", items[1].PrimaryImage.URL)
	}
}

func TestRatingByID(t *testing.T) {
	srv := newTestServer(map[string]string{
		"tt11378946||": `{"Title":"Michael","Type":"movie","Metascore":"55","imdbRating":"7.4",
			"imdbVotes":"157,000","imdbID":"tt11378946","Response":"True"}`,
	})
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	rating, meta, err := p.RatingByID(context.Background(), "tt11378946")
	if err != nil {
		t.Fatalf("RatingByID: %v", err)
	}
	if rating.AggregateRating != 7.4 || rating.VoteCount != 157000 {
		t.Fatalf("rating %+v", rating)
	}
	if meta == nil || meta.Score != 55 {
		t.Fatalf("metacritic %+v", meta)
	}
	// RatingByID must NOT trigger season fetches (only the single ?i= call).
}
