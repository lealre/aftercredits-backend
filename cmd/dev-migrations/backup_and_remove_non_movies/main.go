package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
)

const backupFileName = "backup_non_movie_titles.json"

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

	fmt.Println("🔄 Starting migration: Backup and remove non-movie titles...")

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

	// Step 2: Save titles to JSON file
	fmt.Println("📝 Step 2: Saving non-movie titles to backup file...")
	backupData, err := json.MarshalIndent(nonMovieTitles, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal titles to JSON: %v", err)
	}

	if err := os.WriteFile(backupFileName, backupData, 0644); err != nil {
		log.Fatalf("Failed to write backup file: %v", err)
	}

	fmt.Printf("✅ Saved %d titles to %s\n", len(nonMovieTitles), backupFileName)

	// Step 3: Collect all title IDs to remove
	titleIdsToRemove := make([]string, 0, len(nonMovieTitles))
	for _, title := range nonMovieTitles {
		titleIdsToRemove = append(titleIdsToRemove, title.ID)
	}

	fmt.Printf("📋 Title IDs to remove: %v\n", titleIdsToRemove)

	// Step 4: Remove these title IDs from groups
	fmt.Println("📝 Step 3: Removing non-movie titles from groups...")
	groupsColl := database.Collection(mongodb.GroupsCollection)

	// Remove titles from all groups using $pull
	groupsResult, err := groupsColl.UpdateMany(
		ctx,
		bson.M{},
		bson.M{
			"$pull": bson.M{
				"titles": bson.M{
					"titleId": bson.M{"$in": titleIdsToRemove},
				},
			},
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to remove titles from groups: %v", err)
	}

	fmt.Printf("✅ Updated %d groups (removed non-movie titles from titles array)\n", groupsResult.ModifiedCount)

	// Step 5: Delete titles from titles collection
	fmt.Println("📝 Step 4: Deleting non-movie titles from titles collection...")
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
	fmt.Printf("   - Found and backed up %d non-movie titles\n", len(nonMovieTitles))
	fmt.Printf("   - Backup saved to: %s\n", backupFileName)
	fmt.Printf("   - Removed titles from %d groups\n", groupsResult.ModifiedCount)
	fmt.Printf("   - Deleted %d titles from titles collection\n", titlesDeleteResult.DeletedCount)
	fmt.Println("\n⚠️  Important:")
	fmt.Printf("   - Backup file location: %s\n", backupFileName)
	fmt.Println("   - You can restore titles from the backup file if needed")
	fmt.Println("   - The backup contains the full TitleDb structure for easy restoration")
}
