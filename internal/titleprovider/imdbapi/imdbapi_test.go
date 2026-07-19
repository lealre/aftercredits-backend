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
