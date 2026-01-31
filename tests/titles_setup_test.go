package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

const MOVIE_TILES_FIXTURES_PATH = "fixtures/movieTitles.json"
const TV_SERIES_TILES_FIXTURES_PATH = "fixtures/tvSeriesTitles.json"

func seedTitles(t *testing.T, titles []imdb.Title) {
	t.Helper()

	ctx := context.Background()
	coll := testClient.Database(TEST_DB_NAME).Collection(mongodb.TitlesCollection)

	docs := make([]interface{}, len(titles))
	for i := range titles {
		docs[i] = titles[i]
	}

	_, err := coll.InsertMany(ctx, docs)
	if err != nil {
		t.Fatalf("failed to insert seed titles: %v", err)
	}
}

func loadTitlesFixture(t *testing.T) []imdb.Title {
	t.Helper()

	absPath, err := filepath.Abs(MOVIE_TILES_FIXTURES_PATH)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", absPath, err)
	}

	var docs []imdb.Title
	if err := json.Unmarshal(data, &docs); err != nil {
		t.Fatalf("failed to unmarshal fixture JSON: %v", err)
	}

	return docs
}

func loadTVSeriesTitlesFixture(t *testing.T) []imdb.Title {
	t.Helper()

	absPath, err := filepath.Abs(TV_SERIES_TILES_FIXTURES_PATH)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", absPath, err)
	}

	var docs []imdb.Title
	if err := json.Unmarshal(data, &docs); err != nil {
		t.Fatalf("failed to unmarshal fixture JSON: %v", err)
	}

	return docs
}

func getTitles(t *testing.T) []mongodb.TitleDb {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.TitlesCollection)

	cursor, err := coll.Find(ctx, bson.M{})
	require.NoError(t, err, "error querying titles from db")
	defer cursor.Close(ctx)

	var titles []mongodb.TitleDb
	err = cursor.All(ctx, &titles)
	require.NoError(t, err, "error decoding titles from db")

	return titles
}
