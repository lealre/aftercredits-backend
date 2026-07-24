package main

import (
	"context"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
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

	provider, err := factory.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to build title provider: %v", err)
	}
	log.Printf("Using title provider: %s", provider.Name())

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	collection := db.Collection(mongodb.TitlesCollection)

	log.Println("Fetching all title IDs from database...")
	titleIDs, err := getAllTitleIDs(ctx, collection)
	if err != nil {
		log.Fatalf("Failed to fetch title IDs: %v", err)
	}
	log.Printf("Found %d titles to sync", len(titleIDs))

	if err := syncTitles(ctx, provider, collection, titleIDs); err != nil {
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

func syncTitles(ctx context.Context, provider titleprovider.Provider, collection *mongo.Collection, titleIDs []string) error {
	jobs := make(chan string, len(titleIDs))
	wg := sync.WaitGroup{}
	workerCount := 5

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for titleID := range jobs {
				if err := processTitle(ctx, provider, collection, titleID); err != nil {
					log.Printf("failed processing %s: %v", titleID, err)
				}
			}
		}()
	}

	for _, id := range titleIDs {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	return nil
}

func processTitle(ctx context.Context, provider titleprovider.Provider, collection *mongo.Collection, titleID string) error {
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
		"_id": 1, "type": 1, "primaryImage": 1, "seasons": 1,
		"episodes": 1, "rating": 1, "metacritic": 1,
	}

	err := collection.FindOne(ctx, bson.M{"_id": titleID}, options.FindOne().SetProjection(projection)).Decode(&dbTitle)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Title %s not found in database, skipping", titleID)
			return nil
		}
		return err
	}

	apiTitle, err := provider.GetTitle(ctx, titleID)
	if err != nil {
		return err
	}

	updateDoc := bson.M{}
	hasChanges := false

	apiPrimaryImage := mongodb.Image{
		URL:    apiTitle.PrimaryImage.URL,
		Width:  apiTitle.PrimaryImage.Width,
		Height: apiTitle.PrimaryImage.Height,
	}
	if !imagesEqual(dbTitle.PrimaryImage, apiPrimaryImage) {
		updateDoc["primaryImage"] = apiPrimaryImage
		hasChanges = true
	}

	apiSeasons := titles.MapImdbSeasonsToDbSeasons(apiTitle.Seasons)
	if !reflect.DeepEqual(dbTitle.Seasons, apiSeasons) {
		updateDoc["seasons"] = apiSeasons
		hasChanges = true
	}

	apiEpisodes := titles.MapImdbEpisodesToDbEpisodes(apiTitle.Episodes)
	if !reflect.DeepEqual(dbTitle.Episodes, apiEpisodes) {
		updateDoc["episodes"] = apiEpisodes
		hasChanges = true
	}

	apiRating := mongodb.Rating{
		AggregateRating: apiTitle.Rating.AggregateRating,
		VoteCount:       apiTitle.Rating.VoteCount,
	}
	if dbTitle.Rating != apiRating {
		updateDoc["rating"] = apiRating
		hasChanges = true
	}

	var apiMetacritic *mongodb.Metacritic
	if apiTitle.Metacritic != nil {
		apiMetacritic = &mongodb.Metacritic{Score: apiTitle.Metacritic.Score, ReviewCount: apiTitle.Metacritic.ReviewCount}
	}
	if !metacriticEqual(dbTitle.Metacritic, apiMetacritic) {
		updateDoc["metacritic"] = apiMetacritic
		hasChanges = true
	}

	updateDoc["updatedAt"] = time.Now()

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": titleID}, bson.M{"$set": updateDoc}); err != nil {
		return err
	}

	if hasChanges {
		log.Printf("Updated title %s (fields changed)", titleID)
	} else {
		log.Printf("Updated title %s (updatedAt only)", titleID)
	}
	return nil
}

func imagesEqual(a, b mongodb.Image) bool {
	return a.URL == b.URL && a.Width == b.Width && a.Height == b.Height
}

func metacriticEqual(a, b *mongodb.Metacritic) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Score == b.Score && a.ReviewCount == b.ReviewCount
}
