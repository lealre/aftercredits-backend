package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	database := dbClient.Database(db.GetDatabaseName())

	fmt.Println("🔄 Starting migration: Remove all non-movie titles...")

	// Step 1: Find all titles that are NOT "movie" type
	fmt.Println("📝 Step 1: Finding all non-movie titles...")
	titlesColl := database.Collection(mongodb.TitlesCollection)

	filter := bson.M{"type": bson.M{"$ne": "movie"}}
	cursor, err := titlesColl.Find(ctx, filter)
	if err != nil {
		log.Fatalf("Failed to find non-movie titles: %v", err)
	}
	defer cursor.Close(ctx)

	var nonMovieTitles []mongodb.TitleDb
	if err := cursor.All(ctx, &nonMovieTitles); err != nil {
		log.Fatalf("Failed to decode non-movie titles: %v", err)
	}

	fmt.Printf("✅ Found %d non-movie titles\n", len(nonMovieTitles))

	if len(nonMovieTitles) == 0 {
		fmt.Println("ℹ️  No non-movie titles found. Nothing to do.")
		return
	}

	// Step 2: Collect all title IDs to remove
	titleIdsToRemove := make([]string, 0, len(nonMovieTitles))
	for _, title := range nonMovieTitles {
		titleIdsToRemove = append(titleIdsToRemove, title.ID)
	}

	fmt.Printf("📋 Title IDs to remove: %v\n", titleIdsToRemove)

	// Step 3: Remove these title IDs from groups
	fmt.Println("📝 Step 2: Removing non-movie titles from groups...")
	groupsColl := database.Collection(mongodb.GroupsCollection)

	// Build $unset document for all title IDs (titles are stored as a map, not an array)
	unsetDoc := bson.M{}
	for _, titleId := range titleIdsToRemove {
		unsetDoc[fmt.Sprintf("titles.%s", titleId)] = ""
	}

	// Remove titles from all groups using $unset (since titles is now a map)
	groupsResult, err := groupsColl.UpdateMany(
		ctx,
		bson.M{},
		bson.M{
			"$unset": unsetDoc,
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to remove titles from groups: %v", err)
	}

	fmt.Printf("✅ Updated %d groups (removed non-movie titles from titles map)\n", groupsResult.ModifiedCount)

	// Step 4: Delete titles from titles collection
	fmt.Println("📝 Step 3: Deleting non-movie titles from titles collection...")
	titlesDeleteResult, err := titlesColl.DeleteMany(
		ctx,
		bson.M{"_id": bson.M{"$in": titleIdsToRemove}},
	)
	if err != nil {
		log.Fatalf("Failed to delete titles: %v", err)
	}

	fmt.Printf("✅ Deleted %d titles from titles collection\n", titlesDeleteResult.DeletedCount)

	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Println("\n📋 Summary:")
	fmt.Printf("   - Found %d non-movie titles\n", len(nonMovieTitles))
	fmt.Printf("   - Removed titles from %d groups\n", groupsResult.ModifiedCount)
	fmt.Printf("   - Deleted %d titles from titles collection\n", titlesDeleteResult.DeletedCount)
}



