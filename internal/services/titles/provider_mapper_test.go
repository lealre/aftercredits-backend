package titles

import (
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

func TestMapProviderTitleToDb(t *testing.T) {
	runtime := 3540
	plot := "ep plot"
	src := titleprovider.Title{
		ID:              "tt0903747",
		Type:            "tvSeries",
		PrimaryTitle:    "Breaking Bad",
		PrimaryImage:    titleprovider.Image{URL: "u", Width: 1, Height: 2},
		StartYear:       2008,
		RuntimeSeconds:  2820,
		Genres:          []string{"Drama"},
		Rating:          titleprovider.Rating{AggregateRating: 8.9, VoteCount: 200},
		Plot:            "series plot",
		Directors:       []titleprovider.Person{{ID: "1", DisplayName: "D"}},
		OriginCountries: []titleprovider.CodeName{{Code: "US", Name: "United States"}},
		Seasons:         []titleprovider.Season{{Season: "1", EpisodeCount: 1}},
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
