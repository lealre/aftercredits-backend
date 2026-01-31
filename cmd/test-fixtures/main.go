package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/lealre/movies-backend/internal/imdb"
)

func main() {
	movieTitles := []string{"tt0068646", "tt0075148", "tt1092016", "tt0381707", "tt0133093"}
	tvSeriesTitles := []string{"tt1190634", "tt0903747"}

	// Fetch movie titles
	movieTitlesToExport := make([]imdb.Title, len(movieTitles))
	for i, titleID := range movieTitles {
		log.Printf("Fetching movie title: %s", titleID)
		title, err := fetchTitle(titleID)
		if err != nil {
			log.Fatalf("Error fetching movie title %s: %v", titleID, err)
		}
		movieTitlesToExport[i] = title
	}

	// Fetch TV series titles with seasons and episodes
	tvSeriesTitlesToExport := make([]imdb.Title, len(tvSeriesTitles))
	for i, titleID := range tvSeriesTitles {
		log.Printf("Fetching TV series title: %s", titleID)
		title, err := fetchTitleWithSeasonsAndEpisodes(titleID)
		if err != nil {
			log.Fatalf("Error fetching TV series title %s: %v", titleID, err)
		}
		tvSeriesTitlesToExport[i] = title
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Write movie titles fixture
	moviePath := filepath.Join(rootDir, "tests/fixtures/movieTitles.json")
	if err := writeFixture(moviePath, movieTitlesToExport); err != nil {
		log.Fatalf("Error writing movie titles fixture: %v", err)
	}
	log.Printf("Successfully created movie titles fixture: %s", moviePath)

	// Write TV series titles fixture
	tvSeriesPath := filepath.Join(rootDir, "tests/fixtures/tvSeriesTitles.json")
	if err := writeFixture(tvSeriesPath, tvSeriesTitlesToExport); err != nil {
		log.Fatalf("Error writing TV series titles fixture: %v", err)
	}
	log.Printf("Successfully created TV series titles fixture: %s", tvSeriesPath)
}

// fetchTitle fetches a title from the IMDB API
func fetchTitle(titleID string) (imdb.Title, error) {
	resp, err := imdb.FetchTitle(titleID)
	if err != nil {
		return imdb.Title{}, err
	}

	var imdbTitle imdb.Title
	if err = json.Unmarshal(resp, &imdbTitle); err != nil {
		return imdb.Title{}, err
	}

	return imdbTitle, nil
}

// fetchTitleWithSeasonsAndEpisodes fetches a TV series title with its seasons and episodes
func fetchTitleWithSeasonsAndEpisodes(titleID string) (imdb.Title, error) {
	// Fetch the title
	title, err := fetchTitle(titleID)
	if err != nil {
		return imdb.Title{}, err
	}

	// Fetch seasons
	log.Printf("  Fetching seasons for %s", titleID)
	seasonsResp, err := imdb.FetchSeasons(titleID)
	if err != nil {
		return imdb.Title{}, err
	}

	var seasons imdb.SeasonsResponse
	if err = json.Unmarshal(seasonsResp, &seasons); err != nil {
		return imdb.Title{}, err
	}
	title.Seasons = &seasons.Seasons

	// Fetch all episodes with pagination
	log.Printf("  Fetching episodes for %s", titleID)
	allEpisodes := []imdb.Episode{}
	pageSize := 50
	pageToken := ""

	for {
		episodesResp, err := imdb.FetchEpisodes(titleID, pageSize, pageToken)
		if err != nil {
			return imdb.Title{}, err
		}

		var episodes imdb.EpisodesResponse
		if err = json.Unmarshal(episodesResp, &episodes); err != nil {
			return imdb.Title{}, err
		}

		allEpisodes = append(allEpisodes, episodes.Episodes...)

		// If there's no next page token, we're done
		if episodes.NextPageToken == "" {
			break
		}

		pageToken = episodes.NextPageToken
	}
	title.Episodes = &allEpisodes

	return title, nil
}

// writeFixture writes the data to a JSON file
func writeFixture(filePath string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return err
	}

	return nil
}
