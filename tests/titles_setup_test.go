package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	titles, _, err := testStore.GetTitlesPage(context.Background(), nil, "", nil, 1000, 1)
	require.NoError(t, err, "error querying titles from db")
	return titles
}
