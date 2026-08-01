package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/models"
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
		movieTitlesToExport[i] = modelTitleToDb(titles.MapProviderTitleToDb(*t))
	}

	tvSeriesTitlesToExport := make([]mongodb.TitleDb, len(tvSeriesTitles))
	for i, titleID := range tvSeriesTitles {
		log.Printf("Fetching TV series title: %s", titleID)
		t, err := provider.GetTitle(ctx, titleID)
		if err != nil {
			log.Fatalf("Error fetching TV series title %s: %v", titleID, err)
		}
		tvSeriesTitlesToExport[i] = modelTitleToDb(titles.MapProviderTitleToDb(*t))
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

// modelTitleToDb converts the storage-neutral models.Title produced by
// titles.MapProviderTitleToDb back into the mongo-specific TitleDb shape, so
// the exported fixture JSON keeps using the collection's json tags (camelCase
// field names) that tests decode into mongodb.TitleDb.
func modelTitleToDb(t models.Title) mongodb.TitleDb {
	db := mongodb.TitleDb{
		ID:              t.ID,
		Type:            t.Type,
		PrimaryTitle:    t.PrimaryTitle,
		PrimaryImage:    imageModelToDb(t.PrimaryImage),
		StartYear:       t.StartYear,
		RuntimeSeconds:  t.RuntimeSeconds,
		Genres:          t.Genres,
		Rating:          ratingModelToDb(t.Rating),
		Plot:            t.Plot,
		Directors:       personsModelToDb(t.Directors),
		Writers:         personsModelToDb(t.Writers),
		Stars:           personsModelToDb(t.Stars),
		OriginCountries: codeNamesModelToDb(t.OriginCountries),
		SpokenLanguages: codeNamesModelToDb(t.SpokenLanguages),
		Interests:       interestsModelToDb(t.Interests),
		Seasons:         seasonsModelToDb(t.Seasons),
		Episodes:        episodesModelToDb(t.Episodes),
		AddedAt:         t.AddedAt,
		UpdatedAt:       t.UpdatedAt,
	}
	if t.Metacritic != nil {
		db.Metacritic = &mongodb.Metacritic{Score: t.Metacritic.Score, ReviewCount: t.Metacritic.ReviewCount}
	}
	return db
}

func imageModelToDb(i models.Image) mongodb.Image {
	return mongodb.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func ratingModelToDb(r models.Rating) mongodb.Rating {
	return mongodb.Rating{AggregateRating: r.AggregateRating, VoteCount: r.VoteCount}
}

func codeNamesModelToDb(cs []models.CodeName) []mongodb.CodeName {
	out := make([]mongodb.CodeName, len(cs))
	for i, c := range cs {
		out[i] = mongodb.CodeName{Code: c.Code, Name: c.Name}
	}
	return out
}

func interestsModelToDb(is []models.Interest) []mongodb.Interest {
	out := make([]mongodb.Interest, len(is))
	for i, v := range is {
		out[i] = mongodb.Interest{ID: v.ID, Name: v.Name, IsSubgenre: v.IsSubgenre}
	}
	return out
}

func personsModelToDb(ps []models.Person) []mongodb.Person {
	out := make([]mongodb.Person, len(ps))
	for i, p := range ps {
		person := mongodb.Person{
			ID:                 p.ID,
			DisplayName:        p.DisplayName,
			AlternativeNames:   p.AlternativeNames,
			PrimaryProfessions: p.PrimaryProfessions,
		}
		if p.PrimaryImage != nil {
			img := imageModelToDb(*p.PrimaryImage)
			person.PrimaryImage = &img
		}
		out[i] = person
	}
	return out
}

func seasonsModelToDb(ss []models.Seasons) []mongodb.Seasons {
	out := make([]mongodb.Seasons, len(ss))
	for i, s := range ss {
		out[i] = mongodb.Seasons{Season: s.Season, EpisodeCount: s.EpisodeCount}
	}
	return out
}

func episodesModelToDb(es []models.Episode) []mongodb.Episode {
	out := make([]mongodb.Episode, len(es))
	for i, e := range es {
		ep := mongodb.Episode{
			ID:             e.ID,
			Title:          e.Title,
			PrimaryImage:   imageModelToDb(e.PrimaryImage),
			Season:         e.Season,
			EpisodeNumber:  e.EpisodeNumber,
			RuntimeSeconds: e.RuntimeSeconds,
			Plot:           e.Plot,
		}
		if e.Rating != nil {
			ep.Rating = &mongodb.Rating{AggregateRating: e.Rating.AggregateRating, VoteCount: e.Rating.VoteCount}
		}
		if e.ReleaseDate != nil {
			ep.ReleaseDate = &mongodb.ReleaseDate{Year: e.ReleaseDate.Year, Month: e.ReleaseDate.Month, Day: e.ReleaseDate.Day}
		}
		out[i] = ep
	}
	return out
}
