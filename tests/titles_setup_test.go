package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
)

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

func loadTitlesFixture(t *testing.T, path string) []imdb.Title {
	t.Helper()

	absPath, err := filepath.Abs(path)
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
