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

func main() {
	// Load environment variables from .env if present
	_ = godotenv.Load()

	ctx := context.Background()

	// Connect to MongoDB
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	database := dbClient.Database(db.GetDatabaseName())

	timestamp := time.Now().Format("20060102150405")

	fmt.Println("🔄 Starting migration: Backup and remove ratings and comments for NON-MOVIE titles...")

	// Step 1: Find all titles that are NOT "movie" type
	fmt.Println("📝 Step 1: Finding all non-movie titles...")
	titlesColl := database.Collection(mongodb.TitlesCollection)

	titleFilter := bson.M{"type": bson.M{"$ne": "movie"}}
	titleCursor, err := titlesColl.Find(ctx, titleFilter)
	if err != nil {
		log.Fatalf("Failed to find non-movie titles: %v", err)
	}
	defer titleCursor.Close(ctx)

	var nonMovieTitles []mongodb.TitleDb
	if err := titleCursor.All(ctx, &nonMovieTitles); err != nil {
		log.Fatalf("Failed to decode non-movie titles: %v", err)
	}

	if len(nonMovieTitles) == 0 {
		fmt.Println("ℹ️  No non-movie titles found. Nothing to do.")
		return
	}

	fmt.Printf("✅ Found %d non-movie titles\n", len(nonMovieTitles))

	// Collect all non-movie title IDs
	titleIds := make([]string, 0, len(nonMovieTitles))
	for _, title := range nonMovieTitles {
		titleIds = append(titleIds, title.ID)
	}

	// ============================
	// Backup and remove RATINGS
	// ============================
	fmt.Println("📝 Step 2: Backing up ratings for non-movie titles...")
	ratingsColl := database.Collection(mongodb.RatingsCollection)

	ratingsFilter := bson.M{"titleId": bson.M{"$in": titleIds}}
	ratingsCursor, err := ratingsColl.Find(ctx, ratingsFilter)
	if err != nil {
		log.Fatalf("Failed to find ratings for non-movie titles: %v", err)
	}
	defer ratingsCursor.Close(ctx)

	var ratings []bson.M
	if err := ratingsCursor.All(ctx, &ratings); err != nil {
		log.Fatalf("Failed to decode ratings: %v", err)
	}

	fmt.Printf("✅ Found %d ratings for non-movie titles\n", len(ratings))

	var ratingsBackupFileName string
	if len(ratings) > 0 {
		ratingsBackupData, err := json.MarshalIndent(ratings, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal ratings to JSON: %v", err)
		}

		ratingsBackupFileName = fmt.Sprintf("backup_non_movie_ratings_%s.json", timestamp)
		if err := os.WriteFile(ratingsBackupFileName, ratingsBackupData, 0644); err != nil {
			log.Fatalf("Failed to write ratings backup file: %v", err)
		}

		fmt.Printf("✅ Saved %d ratings to %s\n", len(ratings), ratingsBackupFileName)

		fmt.Println("🧹 Step 3: Deleting ratings for non-movie titles from collection...")
		ratingsDeleteResult, err := ratingsColl.DeleteMany(ctx, ratingsFilter)
		if err != nil {
			log.Fatalf("Failed to delete ratings: %v", err)
		}

		fmt.Printf("✅ Deleted %d ratings documents\n", ratingsDeleteResult.DeletedCount)
	} else {
		fmt.Println("ℹ️  No ratings found for non-movie titles. Skipping backup and delete for ratings.")
	}

	// ============================
	// Backup and remove COMMENTS
	// ============================
	fmt.Println("📝 Step 4: Backing up comments for non-movie titles...")
	commentsColl := database.Collection(mongodb.CommentsCollection)

	commentsFilter := bson.M{"titleId": bson.M{"$in": titleIds}}
	commentsCursor, err := commentsColl.Find(ctx, commentsFilter)
	if err != nil {
		log.Fatalf("Failed to find comments for non-movie titles: %v", err)
	}
	defer commentsCursor.Close(ctx)

	var comments []bson.M
	if err := commentsCursor.All(ctx, &comments); err != nil {
		log.Fatalf("Failed to decode comments: %v", err)
	}

	fmt.Printf("✅ Found %d comments for non-movie titles\n", len(comments))

	var commentsBackupFileName string
	if len(comments) > 0 {
		commentsBackupData, err := json.MarshalIndent(comments, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal comments to JSON: %v", err)
		}

		commentsBackupFileName = fmt.Sprintf("backup_non_movie_comments_%s.json", timestamp)
		if err := os.WriteFile(commentsBackupFileName, commentsBackupData, 0644); err != nil {
			log.Fatalf("Failed to write comments backup file: %v", err)
		}

		fmt.Printf("✅ Saved %d comments to %s\n", len(comments), commentsBackupFileName)

		fmt.Println("🧹 Step 5: Deleting comments for non-movie titles from collection...")
		commentsDeleteResult, err := commentsColl.DeleteMany(ctx, commentsFilter)
		if err != nil {
			log.Fatalf("Failed to delete comments: %v", err)
		}

		fmt.Printf("✅ Deleted %d comments documents\n", commentsDeleteResult.DeletedCount)
	} else {
		fmt.Println("ℹ️  No comments found for non-movie titles. Skipping backup and delete for comments.")
	}

	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Println("\n📋 Summary:")
	fmt.Printf("   - Found %d non-movie titles\n", len(nonMovieTitles))
	fmt.Printf("   - Found and backed up %d ratings for non-movie titles\n", len(ratings))
	if ratingsBackupFileName != "" {
		fmt.Printf("   - Ratings backup saved to: %s\n", ratingsBackupFileName)
	}
	fmt.Printf("   - Found and backed up %d comments for non-movie titles\n", len(comments))
	if commentsBackupFileName != "" {
		fmt.Printf("   - Comments backup saved to: %s\n", commentsBackupFileName)
	}
	fmt.Println("\n⚠️  Important:")
	if ratingsBackupFileName != "" {
		fmt.Printf("   - Ratings backup file location: %s\n", ratingsBackupFileName)
	}
	if commentsBackupFileName != "" {
		fmt.Printf("   - Comments backup file location: %s\n", commentsBackupFileName)
	}
	fmt.Println("   - You can restore ratings/comments from the backup files if needed")
}
