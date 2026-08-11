package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/stretchr/testify/require"
)

const MOVIE_TILES_FIXTURES_PATH = "fixtures/movieTitles.json"
const TV_SERIES_TILES_FIXTURES_PATH = "fixtures/tvSeriesTitles.json"

func seedTitles(t *testing.T, titles []models.Title) {
	t.Helper()
	ctx := context.Background()
	for _, tt := range titles {
		if err := testStore.AddTitle(ctx, tt); err != nil {
			t.Fatalf("failed to insert seed title %s: %v", tt.ID, err)
		}
	}
}

// newSortableMovieTitle builds a minimal, valid movie title whose sort columns
// (primary_title, start_year, rating_aggregate, vote_count, updated_at) are all
// caller-controlled, so a test can deliberately create ties on any of them. The
// id must be IMDb-shaped ("tt" + digits) because the add-title-to-group
// endpoint parses it out of an IMDb URL.
//
// updatedAt is a pointer on purpose: updated_at is the one nullable column in
// the sort whitelist, and NULL rows tie with each other just like equal values
// do.
func newSortableMovieTitle(
	id, primaryTitle string,
	startYear int,
	rating float64,
	voteCount int,
	updatedAt *time.Time,
) models.Title {
	addedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return models.Title{
		ID:             id,
		Type:           "movie",
		PrimaryTitle:   primaryTitle,
		PrimaryImage:   models.Image{URL: "https://example.com/" + id + ".jpg", Width: 100, Height: 150},
		StartYear:      startYear,
		RuntimeSeconds: 7200,
		Genres:         []string{"Drama"},
		Rating:         models.Rating{AggregateRating: rating, VoteCount: voteCount},
		Plot:           "plot for " + id,
		AddedAt:        &addedAt,
		UpdatedAt:      updatedAt,
	}
}

func loadTitlesFixture(t *testing.T) []models.Title {
	t.Helper()

	absPath, err := filepath.Abs(MOVIE_TILES_FIXTURES_PATH)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", absPath, err)
	}

	var docs []models.Title
	if err := json.Unmarshal(data, &docs); err != nil {
		t.Fatalf("failed to unmarshal fixture JSON: %v", err)
	}

	return docs
}

func loadTVSeriesTitlesFixture(t *testing.T) []models.Title {
	t.Helper()

	absPath, err := filepath.Abs(TV_SERIES_TILES_FIXTURES_PATH)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", absPath, err)
	}

	var docs []models.Title
	if err := json.Unmarshal(data, &docs); err != nil {
		t.Fatalf("failed to unmarshal fixture JSON: %v", err)
	}

	return docs
}

func getTitles(t *testing.T) []models.Title {
	titles, _, err := testStore.GetTitlesPage(context.Background(), "", nil, 1000, 1)
	require.NoError(t, err, "error querying titles from db")
	return titles
}
