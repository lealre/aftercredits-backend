package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/users"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	indexes := flag.Bool("indexes", false, "create indexes in the database if they do not exist")
	resetIndexes := flag.Bool("reset", false, "Delete the indexes and recreate it")
	deleteIndexes := flag.Bool("delete", false, "Delete the indexes")
	superuser := flag.Bool("superuser", false, "create a superuser if it does not exist")

	flag.Parse()

	switch {
	case *indexes:
		if *deleteIndexes {
			if err := deleteAllIndexes(ctx, database); err != nil {
				log.Fatalf("Failed to delete indexes: %v", err)
			}
			fmt.Println("✅ All indexes deleted successfully!")
			return
		}

		if err := createAllIndexes(ctx, database, *resetIndexes); err != nil {
			log.Fatalf("Failed to create indexes: %v", err)
		}
		fmt.Println("✅ indexes comman ran successfully!")

	case *superuser:
		if err := createSuperuser(ctx, db); err != nil {
			log.Fatalf("Failed to create superuser: %v", err)
		}
		fmt.Println("✅ Superuser command ran successfully!")

	default:
		fmt.Println("No valid command specified.")
		flag.Usage()
	}

}

func deleteAllIndexes(ctx context.Context, db *mongo.Database) error {
	// Get all collections in the database
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, collName := range collections {
		coll := db.Collection(collName)

		// List all indexes for this collection
		cursor, err := coll.Indexes().List(ctx)
		if err != nil {
			return fmt.Errorf("failed to list indexes for collection '%s': %w", collName, err)
		}

		// Iterate through indexes and delete them (except _id_ which is the default and cannot be deleted)
		for cursor.Next(ctx) {
			var index bson.M
			if err := cursor.Decode(&index); err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("failed to decode index for collection '%s': %w", collName, err)
			}

			indexName, ok := index["name"].(string)
			if !ok {
				continue
			}

			// Skip the default _id_ index as it cannot be deleted
			if indexName == "_id_" {
				continue
			}

			// Delete the index
			_, err := coll.Indexes().DropOne(ctx, indexName)
			if err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("failed to delete index '%s' from collection '%s': %w", indexName, collName, err)
			}
			fmt.Printf("🗑️  Deleted index '%s' from collection '%s'\n", indexName, collName)
		}

		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return fmt.Errorf("cursor error for collection '%s': %w", collName, err)
		}
		cursor.Close(ctx)
	}

	return nil
}

func createAllIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	// Create indexes for users collection
	if err := createUserIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create user indexes: %w", err)
	}

	// Create indexes for ratings collection
	if err := createRatingIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create rating indexes: %w", err)
	}

	// Create indexes for comments collection
	if err := createCommentIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create comment indexes: %w", err)
	}

	return nil
}

func createUserIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(mongodb.UsersCollection)
	usersEmailIndexName := "email_unique"

	// Create unique index on email (case-insensitive)
	// Exclude empty strings and null values from uniqueness constraint
	emailIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(usersEmailIndexName).
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{
				"$and": []bson.M{
					{"email": bson.M{"$type": "string"}},
					{"email": bson.M{"$gt": ""}},
				},
			}),
	}
	if err := createIndexIfNotExists(ctx, coll, emailIndex, usersEmailIndexName, reset); err != nil {
		return err
	}

	// Create unique index on username (case-insensitive)
	// Exclude empty strings and null values from uniqueness constraint
	usersUsernameIndexName := "username_unique"
	usernameIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(usersUsernameIndexName).
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{
				"$and": []bson.M{
					{"username": bson.M{"$type": "string"}},
					{"username": bson.M{"$gt": ""}},
				},
			}),
	}
	if err := createIndexIfNotExists(ctx, coll, usernameIndex, usersUsernameIndexName, reset); err != nil {
		return err
	}

	return nil
}

func createRatingIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(mongodb.RatingsCollection)
	ratingsIndexName := "userId_and_titleId_unique"

	// Create unique index on userId and titleId
	ratingsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(ratingsIndexName),
	}
	if err := createIndexIfNotExists(ctx, coll, ratingsIndex, ratingsIndexName, reset); err != nil {
		return err
	}

	return nil
}

func createCommentIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(mongodb.CommentsCollection)
	commentsIndexName := "userId_and_titleId_unique"

	// Create unique index on userId and titleId
	commentsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(commentsIndexName),
	}
	if err := createIndexIfNotExists(ctx, coll, commentsIndex, commentsIndexName, reset); err != nil {
		return err
	}

	return nil
}

// createIndexIfNotExists checks if an index exists and creates it if it doesn't
func createIndexIfNotExists(ctx context.Context, coll *mongo.Collection, indexModel mongo.IndexModel, indexName string, reset bool) error {
	// List existing indexes
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}
	defer cursor.Close(ctx)

	// Check if index already exists
	indexExists := false
	for cursor.Next(ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			return fmt.Errorf("failed to decode index: %w", err)
		}

		if name, ok := index["name"].(string); ok && name == indexName {
			indexExists = true
			break
		}
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	if indexExists {
		if !reset {
			fmt.Printf("ℹ️  Index '%s' already exists on collection '%s', skipping...\n", indexName, coll.Name())
			return nil
		}
		// Delete the existing index
		_, err := coll.Indexes().DropOne(ctx, indexName)
		if err != nil {
			return fmt.Errorf("failed to delete index '%s': %w", indexName, err)
		}
		fmt.Printf("🗑️  Deleted index '%s' on collection '%s'\n", indexName, coll.Name())
	}

	// Create the index
	_, err = coll.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create index '%s': %w", indexName, err)
	}

	fmt.Printf("✅ Created index '%s' on collection '%s'\n", indexName, coll.Name())
	return nil
}

func createSuperuser(ctx context.Context, db *mongodb.DB) error {
	// Read environment variables
	username := strings.TrimSpace(os.Getenv("SUPERUSER_USERNAME"))
	email := strings.TrimSpace(os.Getenv("SUPERUSER_EMAIL"))
	password := os.Getenv("SUPERUSER_PASSWORD")

	// Apply defaults
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin"
	}
	// Email can be empty, so we leave it as is

	// Validate username if provided
	// if username != "" {
	// 	if len(username) < 3 {
	// 		return fmt.Errorf("username must have at least 3 characters")
	// 	}
	// 	if !users.IsValidUsername(username) {
	// 		return fmt.Errorf("username must contain just letters, numbers, '-' or '_'")
	// 	}
	// }

	// Validate email if provided
	if email != "" && !users.IsValidEmail(email) {
		return fmt.Errorf("email format is not valid")
	}

	// Validate password
	// if len(password) < 4 {
	// 	return fmt.Errorf("password must have at least 4 characters")
	// }

	// Check if user already exists
	_, err := db.GetUserByUsernameOrEmail(ctx, username, email)
	if err == nil {
		fmt.Printf("ℹ️  User with username '%s' or email '%s' already exists, skipping creation\n", username, email)
		return nil
	}
	if err != mongodb.ErrRecordNotFound {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user with Admin role
	now := time.Now()
	userDb := mongodb.UserDb{
		Id:           primitive.NewObjectID().Hex(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         mongodb.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Insert user into database
	err = db.AddUser(ctx, userDb)
	if err != nil {
		return fmt.Errorf("failed to add user to database: %w", err)
	}

	fmt.Printf("Superuser created: username='%s', email='%s'\n", username, email)
	return nil
}
