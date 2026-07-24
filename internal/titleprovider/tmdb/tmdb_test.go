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

func TestSearchTitles_SkipsResultWhenExternalIDsCallFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/multi", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"id":238,"media_type":"movie","title":"The Godfather","release_date":"1972-03-14",
			 "poster_path":"/p.jpg","vote_average":8.7,"vote_count":100},
			{"id":1396,"media_type":"tv","name":"Breaking Bad","first_air_date":"2008-01-20",
			 "poster_path":"/bb.jpg","vote_average":8.9,"vote_count":200}]}`))
	})
	// The movie's external_ids lookup fails; the tv one succeeds.
	mux.HandleFunc("/movie/238/external_ids", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/tv/1396/external_ids", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"imdb_id":"tt0903747"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	items, err := p.SearchTitles(context.Background(), "godfather", 5)
	if err != nil {
		t.Fatalf("Search should not error when one external_ids call fails: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (the failing one skipped), got %d: %+v", len(items), items)
	}
	if items[0].ID != "tt0903747" || items[0].Type != "tvSeries" {
		t.Fatalf("item0 %+v", items[0])
	}
}
