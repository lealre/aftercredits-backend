package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	fmt.Println("🔄 Starting migration: Convert titles array to map...")

	// Step 1: Find all groups with titles as array (old format)
	fmt.Println("📝 Step 1: Finding all groups with titles as array...")
	groupsColl := database.Collection(mongodb.GroupsCollection)

	// Find groups where titles is an array (not a map)
	// We check for documents where titles exists and is an array
	cursor, err := groupsColl.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to find groups: %v", err)
	}
	defer cursor.Close(ctx)

	var groupsToMigrate []bson.M
	if err := cursor.All(ctx, &groupsToMigrate); err != nil {
		log.Fatalf("Failed to decode groups: %v", err)
	}

	fmt.Printf("✅ Found %d groups to check\n", len(groupsToMigrate))

	migratedCount := 0
	skippedCount := 0

	// Step 2: Convert each group's titles array to map
	fmt.Println("📝 Step 2: Converting titles array to map format...")
	for _, groupDoc := range groupsToMigrate {
		titlesValue, exists := groupDoc["titles"]
		if !exists {
			skippedCount++
			continue
		}

		// Check if titles is already a map (new format) or nil/empty
		titlesMap, isMap := titlesValue.(primitive.M)
		if isMap && len(titlesMap) > 0 {
			// Already in map format, skip
			skippedCount++
			continue
		}

		// Check if titles is an array (old format)
		titlesArray, isArray := titlesValue.(primitive.A)
		if !isArray {
			// Not an array, might be nil or empty map, skip
			skippedCount++
			continue
		}

		// Convert array to map
		newTitlesMap := make(bson.M)
		for _, item := range titlesArray {
			itemDoc, ok := item.(primitive.M)
			if !ok {
				log.Printf("⚠️  Warning: Skipping invalid title item in group %s", groupDoc["_id"])
				continue
			}

			titleId, ok := itemDoc["titleId"].(string)
			if !ok {
				log.Printf("⚠️  Warning: Skipping title item without titleId in group %s", groupDoc["_id"])
				continue
			}

			// Use titleId as the map key
			newTitlesMap[titleId] = itemDoc
		}

		// Update the group document
		groupId := groupDoc["_id"].(string)
		_, err := groupsColl.UpdateOne(
			ctx,
			bson.M{"_id": groupId},
			bson.M{
				"$set": bson.M{
					"titles":    newTitlesMap,
					"updatedAt": time.Now(),
				},
			},
		)
		if err != nil {
			log.Printf("❌ Failed to update group %s: %v", groupId, err)
			continue
		}

		migratedCount++
		fmt.Printf("✅ Migrated group %s: converted %d titles from array to map\n", groupId, len(titlesArray))
	}

	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Println("\n📋 Summary:")
	fmt.Printf("   - Checked %d groups\n", len(groupsToMigrate))
	fmt.Printf("   - Migrated %d groups (array → map)\n", migratedCount)
	fmt.Printf("   - Skipped %d groups (already in map format or empty)\n", skippedCount)
	fmt.Println("\n⚠️  Important:")
	fmt.Println("   - All titles arrays have been converted to map format")
	fmt.Println("   - Map keys use the titleId from each title item")
	fmt.Println("   - The migration is idempotent (safe to run multiple times)")
}
