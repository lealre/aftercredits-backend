package tests

import (
	"context"
	"encoding/json"
	"os"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// fakeTitleProvider implements titleprovider.Provider for integration tests,
// backed by the on-disk fixtures. It performs no network calls, so the suite
// needs neither TMDB_API_KEY nor connectivity.
type fakeTitleProvider struct {
	byID map[string]models.Title
}

func newFakeTitleProvider() *fakeTitleProvider {
	f := &fakeTitleProvider{byID: map[string]models.Title{}}
	for _, path := range []string{MOVIE_TILES_FIXTURES_PATH, TV_SERIES_TILES_FIXTURES_PATH} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // fixtures optional; GetTitle returns ErrTitleNotFound if absent
		}
		var docs []models.Title
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
