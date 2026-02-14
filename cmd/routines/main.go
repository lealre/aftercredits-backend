package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	_ = godotenv.Load()

	log.Println("")
	log.Println("==========================================")
	log.Println("🎬 Starting titles update...")
	log.Println("==========================================")

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	collection := db.Collection(mongodb.TitlesCollection)

	// Fetch all title IDs using projection to minimize memory usage
	log.Println("Fetching all title IDs from database...")
	titleIDs, err := getAllTitleIDs(ctx, collection)
	if err != nil {
		log.Fatalf("Failed to fetch title IDs: %v", err)
	}

	log.Printf("Found %d titles to sync", len(titleIDs))

	// Sync titles
	if err := SyncTitles(ctx, collection, titleIDs); err != nil {
		log.Fatalf("Failed to sync titles: %v", err)
	}

	log.Println("Sync completed successfully")
}

func getAllTitleIDs(ctx context.Context, collection *mongo.Collection) ([]string, error) {
	opts := options.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var titleIDs []string
	for cursor.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		titleIDs = append(titleIDs, doc.ID)
	}

	return titleIDs, cursor.Err()
}

func SyncTitles(ctx context.Context, collection *mongo.Collection, titleIDs []string) error {
	// Batch titles into groups of 5
	batchSize := 5
	batches := make([][]string, 0)
	for i := 0; i < len(titleIDs); i += batchSize {
		end := i + batchSize
		if end > len(titleIDs) {
			end = len(titleIDs)
		}
		batches = append(batches, titleIDs[i:end])
	}

	jobs := make(chan []string, len(batches))
	wg := sync.WaitGroup{}

	workerCount := 5

	// start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for batch := range jobs {
				if err := processBatch(ctx, collection, batch); err != nil {
					log.Printf("failed processing batch: %v", err)
				}
			}
		}()
	}

	// feed jobs
	for _, batch := range batches {
		jobs <- batch
	}

	close(jobs)
	wg.Wait()

	return nil
}

func processBatch(ctx context.Context, collection *mongo.Collection, titleIDs []string) error {
	// Fetch batch of titles from IMDb API with retry
	body, err := fetchBatchTitlesWithRetry(titleIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch batch from API: %w", err)
	}

	var batchResponse imdb.BatchTitlesResponse
	if err := json.Unmarshal(body, &batchResponse); err != nil {
		return fmt.Errorf("failed to unmarshal batch API response: %w", err)
	}

	// Process each title in the batch
	for _, apiTitle := range batchResponse.Titles {
		if err := processTitle(ctx, collection, apiTitle); err != nil {
			log.Printf("failed processing %s: %v", apiTitle.ID, err)
			// Continue processing other titles in the batch
		}
	}

	return nil
}

func processTitle(ctx context.Context, collection *mongo.Collection, apiTitle imdb.Title) error {
	titleID := apiTitle.ID

	// Fetch title from DB with projection for only needed fields
	var dbTitle struct {
		ID           string              `bson:"_id"`
		Type         string              `bson:"type"`
		PrimaryImage mongodb.Image       `bson:"primaryImage"`
		Seasons      []mongodb.Seasons   `bson:"seasons"`
		Episodes     []mongodb.Episode   `bson:"episodes"`
		Rating       mongodb.Rating      `bson:"rating"`
		Metacritic   *mongodb.Metacritic `bson:"metacritic,omitempty"`
	}

	projection := bson.M{
		"_id":          1,
		"type":         1,
		"primaryImage": 1,
		"seasons":      1,
		"episodes":     1,
		"rating":       1,
		"metacritic":   1,
	}

	err := collection.FindOne(ctx, bson.M{"_id": titleID}, options.FindOne().SetProjection(projection)).Decode(&dbTitle)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Title %s not found in database, skipping", titleID)
			return nil
		}
		return fmt.Errorf("failed to fetch title from DB: %w", err)
	}

	// For TV series, fetch seasons and episodes
	if apiTitle.Type == "tvSeries" || apiTitle.Type == "tvMiniSeries" {
		seasonsBody, err := fetchSeasonsWithRetry(titleID)
		if err != nil {
			log.Printf("Warning: failed to fetch seasons for %s: %v", titleID, err)
		} else {
			var seasonsResp imdb.SeasonsResponse
			if err := json.Unmarshal(seasonsBody, &seasonsResp); err == nil {
				apiTitle.Seasons = &seasonsResp.Seasons
			}
		}

		// Fetch all episodes with pagination
		allEpisodes := []imdb.Episode{}
		pageSize := 50
		pageToken := ""

		for {
			episodesBody, err := fetchEpisodesWithRetry(titleID, pageSize, pageToken)
			if err != nil {
				log.Printf("Warning: failed to fetch episodes for %s: %v", titleID, err)
				break
			}

			var episodesResp imdb.EpisodesResponse
			if err := json.Unmarshal(episodesBody, &episodesResp); err != nil {
				log.Printf("Warning: failed to unmarshal episodes for %s: %v", titleID, err)
				break
			}

			allEpisodes = append(allEpisodes, episodesResp.Episodes...)

			if episodesResp.NextPageToken == "" {
				break
			}

			pageToken = episodesResp.NextPageToken
		}

		if len(allEpisodes) > 0 {
			apiTitle.Episodes = &allEpisodes
		}
	}

	// Compare fields and build update document
	updateDoc := bson.M{}
	hasChanges := false

	// Compare PrimaryImage
	apiPrimaryImage := mongodb.Image{
		URL:    apiTitle.PrimaryImage.URL,
		Width:  apiTitle.PrimaryImage.Width,
		Height: apiTitle.PrimaryImage.Height,
	}
	if !imagesEqual(dbTitle.PrimaryImage, apiPrimaryImage) {
		updateDoc["primaryImage"] = apiPrimaryImage
		hasChanges = true
	}

	// Compare Seasons
	dbSeasons := dbTitle.Seasons
	apiSeasons := []mongodb.Seasons{}
	if apiTitle.Seasons != nil {
		apiSeasons = titles.MapImdbSeasonsToDbSeasons(*apiTitle.Seasons)
	}
	if !seasonsEqual(dbSeasons, apiSeasons) {
		updateDoc["seasons"] = apiSeasons
		hasChanges = true
	}

	// Compare Episodes
	dbEpisodes := dbTitle.Episodes
	apiEpisodes := []mongodb.Episode{}
	if apiTitle.Episodes != nil {
		apiEpisodes = titles.MapImdbEpisodesToDbEpisodes(*apiTitle.Episodes)
	}
	if !episodesEqual(dbEpisodes, apiEpisodes) {
		updateDoc["episodes"] = apiEpisodes
		hasChanges = true
	}

	// Compare Rating
	apiRating := mongodb.Rating{
		AggregateRating: apiTitle.Rating.AggregateRating,
		VoteCount:       apiTitle.Rating.VoteCount,
	}
	if !ratingsEqual(dbTitle.Rating, apiTitle.Rating) {
		updateDoc["rating"] = apiRating
		hasChanges = true
	}

	// Compare Metacritic
	var apiMetacritic *mongodb.Metacritic
	if apiTitle.Metacritic != nil {
		apiMetacritic = &mongodb.Metacritic{
			Score:       apiTitle.Metacritic.Score,
			ReviewCount: apiTitle.Metacritic.ReviewCount,
		}
	}
	if !metacriticEqual(dbTitle.Metacritic, apiTitle.Metacritic) {
		updateDoc["metacritic"] = apiMetacritic
		hasChanges = true
	}

	// Always update updatedAt
	now := time.Now()
	updateDoc["updatedAt"] = now

	// Update MongoDB (always update updatedAt, and other fields if they changed)
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": titleID},
		bson.M{"$set": updateDoc},
	)
	if err != nil {
		return fmt.Errorf("failed to update title: %w", err)
	}

	if hasChanges {
		log.Printf("Updated title %s (fields changed)", titleID)
	} else {
		log.Printf("Updated title %s (updatedAt only)", titleID)
	}

	return nil
}

func fetchBatchTitlesWithRetry(titleIDs []string) ([]byte, error) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		body, err := imdb.FetchBatchTitles(titleIDs)
		if err == nil {
			return body, nil
		}

		// Check if it's a 429 rate limit error
		if strings.Contains(err.Error(), "429") {
			if i < maxRetries-1 {
				log.Printf("Rate limit hit for batch, sleeping for 1 minute before retry %d/%d", i+1, maxRetries-1)
				time.Sleep(1 * time.Minute)
				continue
			}
		}

		return nil, err
	}
	return nil, fmt.Errorf("failed after %d retries", maxRetries)
}

func fetchSeasonsWithRetry(titleID string) ([]byte, error) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		body, err := imdb.FetchSeasons(titleID)
		if err == nil {
			return body, nil
		}

		// Check if it's a 429 rate limit error
		if strings.Contains(err.Error(), "429") {
			if i < maxRetries-1 {
				log.Printf("Rate limit hit for seasons %s, sleeping for 1 minute before retry %d/%d", titleID, i+1, maxRetries-1)
				time.Sleep(1 * time.Minute)
				continue
			}
		}

		return nil, err
	}
	return nil, fmt.Errorf("failed after %d retries", maxRetries)
}

func fetchEpisodesWithRetry(titleID string, pageSize int, pageToken string) ([]byte, error) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		body, err := imdb.FetchEpisodes(titleID, pageSize, pageToken)
		if err == nil {
			return body, nil
		}

		// Check if it's a 429 rate limit error
		if strings.Contains(err.Error(), "429") {
			if i < maxRetries-1 {
				log.Printf("Rate limit hit for episodes %s, sleeping for 1 minute before retry %d/%d", titleID, i+1, maxRetries-1)
				time.Sleep(1 * time.Minute)
				continue
			}
		}

		return nil, err
	}
	return nil, fmt.Errorf("failed after %d retries", maxRetries)
}

func imagesEqual(img1, img2 mongodb.Image) bool {
	return img1.URL == img2.URL && img1.Width == img2.Width && img1.Height == img2.Height
}

func seasonsEqual(s1, s2 []mongodb.Seasons) bool {
	return reflect.DeepEqual(s1, s2)
}

func episodesEqual(e1, e2 []mongodb.Episode) bool {
	return reflect.DeepEqual(e1, e2)
}

func ratingsEqual(r1 mongodb.Rating, r2 imdb.Rating) bool {
	return r1.AggregateRating == r2.AggregateRating && r1.VoteCount == r2.VoteCount
}

func metacriticEqual(m1 *mongodb.Metacritic, m2 *imdb.Metacritic) bool {
	if m1 == nil && m2 == nil {
		return true
	}
	if m1 == nil || m2 == nil {
		return false
	}
	return m1.Score == m2.Score && m1.ReviewCount == m2.ReviewCount
}
