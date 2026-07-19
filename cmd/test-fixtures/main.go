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
