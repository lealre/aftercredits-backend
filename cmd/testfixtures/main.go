package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/lealre/movies-backend/internal/imdb"
)

func main() {
	titles := []string{"tt0068646", "tt0075148", "tt1092016", "tt0381707", "tt0133093"}

	titlesToExport := make([]imdb.Title, len(titles))
	for i, title := range titles {
		resp, err := imdb.FetchTitle(title)
		if err != nil {
			log.Fatal(err)
		}

		var imdbTitle imdb.Title
		if err = json.Unmarshal(resp, &imdbTitle); err != nil {
			log.Fatal(err)
		}
		titlesToExport[i] = imdbTitle
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fullPath := filepath.Join(rootDir, "tests/fixtures/titles.json")

	if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		log.Fatal("could not create directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(titlesToExport, "", "  ")
	if err != nil {
		log.Fatal("could not marshal JSON: %w", err)
	}

	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		log.Fatal("could not write file: %w", err)
	}
}
