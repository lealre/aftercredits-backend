package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/groups"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	dbClient, err := mongodb.Connect(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer dbClient.Disconnect(context.Background())

	db := mongodb.NewDB(dbClient)

	if err := migrateTitlesToGroup(ctx, *db, "690bb4b2029d2b31b8b66835"); err != nil {
		log.Fatal(err)
	}
	// if err := CreateGroup(ctx, *db); err != nil {
	// 	log.Fatal(err)
	// }
	// if err := CopyTitlesToTitlesG(ctx, *db); err != nil {
	// 	log.Fatal(err)
	// }
}

func CopyTitlesToTitlesG(ctx context.Context, db mongodb.DB) error {

	src := db.Collection(mongodb.TitlesCollection)
	dst := db.Collection(mongodb.TitlesCollectionG)

	cursor, err := src.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to find titles: %w", err)
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("failed to decode document: %w", err)
		}

		// Try inserting into titlesG — skip if it already exists
		_, err := dst.InsertOne(ctx, doc)
		if err != nil {
			// Ignore duplicate key errors
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return fmt.Errorf("insert failed: %w", err)
		}
		count++
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	fmt.Printf("✅ Copied %d documents from '%s' to '%s'\n", count, mongodb.TitlesCollection, mongodb.TitlesCollectionG)
	return nil
}

func CreateGroup(ctx context.Context, db mongodb.DB) error {
	groupsCol := db.Collection(mongodb.GroupsCollection)

	timestamp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	newGroupID := primitive.NewObjectID().Hex()

	newGroup := groups.Group{
		Id:        newGroupID,
		Name:      "Renan & Bruna Group",
		Users:     groups.UsersIds{"68e67788956f936302a2a778", "68e67788956f936302a2a779"},
		Titles:    []groups.GroupTitle{}, // no titles yet
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}

	_, err := groupsCol.InsertOne(ctx, newGroup)
	if err != nil {
		return fmt.Errorf("failed to insert group: %w", err)
	}

	fmt.Printf("✅ Group '%s' created with users Renan and Bruna (ID: %s)\n", newGroup.Name, newGroup.Id)
	return nil
}

func migrateTitlesToGroup(ctx context.Context, db mongodb.DB, groupID string) error {
	titlesCol := db.Collection(mongodb.TitlesCollection)
	groupsCol := db.Collection(mongodb.GroupsCollection)

	// 1. Fetch all titles
	cursor, err := titlesCol.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to find titles: %w", err)
	}
	defer cursor.Close(ctx)

	var allTitles []mongodb.TitleDb
	if err := cursor.All(ctx, &allTitles); err != nil {
		return fmt.Errorf("failed to decode titles: %w", err)
	}

	// 2. Convert to groups.GroupTitle format
	var groupTitles []groups.GroupTitle
	now := time.Now()
	for _, t := range allTitles {
		groupTitles = append(groupTitles, groups.GroupTitle{
			Id:        t.ID,
			AddedAt:   derefOrDefault(t.AddedAt, now),
			UpdatedAt: derefOrDefault(t.UpdatedAt, now),
		})
	}

	// 3. Update the target group (replace titles array)
	update := bson.M{
		"$set": bson.M{
			"titles":    groupTitles,
			"updatedAt": now,
		},
	}

	result, err := groupsCol.UpdateByID(ctx, groupID, update)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	fmt.Printf("✅ Migrated %d titles to group %s (matched %d, modified %d)\n",
		len(groupTitles), groupID, result.MatchedCount, result.ModifiedCount)
	return nil
}

func derefOrDefault(ptr *time.Time, fallback time.Time) time.Time {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
