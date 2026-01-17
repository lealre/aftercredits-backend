package main

import (
	"context"
	"fmt"
	"log"
	"strings"

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

	fmt.Println("🔄 Starting migration: Add season field to ratings and comments...")

	// Step 1: Add season field to all existing ratings (default to 0)
	fmt.Println("📝 Step 1: Adding season=0 to all existing ratings...")
	ratingsColl := database.Collection(mongodb.RatingsCollection)
	ratingsResult, err := ratingsColl.UpdateMany(
		ctx,
		bson.M{"season": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"season": 0}},
	)
	if err != nil {
		log.Fatalf("Failed to update ratings: %v", err)
	}
	fmt.Printf("✅ Updated %d ratings with season=0\n", ratingsResult.ModifiedCount)

	// Step 2: Add season field to all existing comments (default to 0)
	fmt.Println("📝 Step 2: Adding season=0 to all existing comments...")
	commentsColl := database.Collection(mongodb.CommentsCollection)
	commentsResult, err := commentsColl.UpdateMany(
		ctx,
		bson.M{"season": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"season": 0}},
	)
	if err != nil {
		log.Fatalf("Failed to update comments: %v", err)
	}
	fmt.Printf("✅ Updated %d comments with season=0\n", commentsResult.ModifiedCount)

	// Step 3: Drop old unique indexes
	fmt.Println("📝 Step 3: Dropping old unique indexes...")

	// Drop old ratings index
	ratingsIndexName := "userId_and_titleId_unique"
	_, err = ratingsColl.Indexes().DropOne(ctx, ratingsIndexName)
	if err != nil && !isIndexNotFoundError(err) {
		log.Fatalf("Failed to drop old ratings index: %v", err)
	}
	if err == nil {
		fmt.Printf("✅ Dropped old ratings index: %s\n", ratingsIndexName)
	} else {
		fmt.Printf("ℹ️  Old ratings index %s not found (may have been already dropped)\n", ratingsIndexName)
	}

	// Drop old comments index
	commentsIndexName := "userId_and_titleId_unique"
	_, err = commentsColl.Indexes().DropOne(ctx, commentsIndexName)
	if err != nil && !isIndexNotFoundError(err) {
		log.Fatalf("Failed to drop old comments index: %v", err)
	}
	if err == nil {
		fmt.Printf("✅ Dropped old comments index: %s\n", commentsIndexName)
	} else {
		fmt.Printf("ℹ️  Old comments index %s not found (may have been already dropped)\n", commentsIndexName)
	}

	// Step 4: Create new indexes with season
	fmt.Println("📝 Step 4: Creating new unique indexes with season field...")
	if err := mongodb.CreateAllIndexes(ctx, database, false); err != nil {
		log.Fatalf("Failed to create new indexes: %v", err)
	}
	fmt.Println("✅ Created new indexes with season field")

	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Println("\n📋 Summary:")
	fmt.Printf("   - Updated %d ratings with season=0\n", ratingsResult.ModifiedCount)
	fmt.Printf("   - Updated %d comments with season=0\n", commentsResult.ModifiedCount)
	fmt.Println("   - Dropped old unique indexes")
	fmt.Println("   - Created new unique indexes (userId, titleId, season)")
	fmt.Println("\n⚠️  Note: If you need to rollback, you would need to:")
	fmt.Println("   1. Drop the new indexes")
	fmt.Println("   2. Remove the season field from all documents")
	fmt.Println("   3. Recreate the old indexes")
}

func isIndexNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// MongoDB returns an error when index is not found
	// Check if error message contains "IndexNotFound" or "index not found"
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "indexnotfound") ||
		strings.Contains(errStr, "index not found") ||
		strings.Contains(errStr, "no such index")
}
