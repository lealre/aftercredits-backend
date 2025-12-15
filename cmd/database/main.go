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
	superuser := flag.Bool("superuser", false, "create a superuser if it does not exist")

	flag.Parse()

	switch {
	case *indexes:
		if err := createAllIndexes(ctx, database); err != nil {
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

func createAllIndexes(ctx context.Context, db *mongo.Database) error {
	// Create indexes for users collection
	if err := createUserIndexes(ctx, db); err != nil {
		return fmt.Errorf("failed to create user indexes: %w", err)
	}

	// Create indexes for ratings collection
	if err := createRatingIndexes(ctx, db); err != nil {
		return fmt.Errorf("failed to create rating indexes: %w", err)
	}

	// Create indexes for comments collection
	if err := createCommentIndexes(ctx, db); err != nil {
		return fmt.Errorf("failed to create comment indexes: %w", err)
	}

	return nil
}

func createUserIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection(mongodb.UsersCollection)

	// Create unique index on email (case-insensitive)
	emailIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("email_1").
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{"email": bson.M{"$exists": true}}),
	}
	if err := createIndexIfNotExists(ctx, coll, emailIndex, "email_1"); err != nil {
		return err
	}

	// Create unique index on username (case-insensitive)
	usernameIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("username_1").
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{"username": bson.M{"$exists": true}}),
	}
	if err := createIndexIfNotExists(ctx, coll, usernameIndex, "username_1"); err != nil {
		return err
	}

	return nil
}

func createRatingIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection(mongodb.RatingsCollection)

	// Create unique index on userId and titleId
	ratingsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("userId_1_titleId_1"),
	}
	if err := createIndexIfNotExists(ctx, coll, ratingsIndex, "userId_1_titleId_1"); err != nil {
		return err
	}

	return nil
}

func createCommentIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection(mongodb.CommentsCollection)

	// Create unique index on userId and titleId
	commentsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("userId_1_titleId_1"),
	}
	if err := createIndexIfNotExists(ctx, coll, commentsIndex, "userId_1_titleId_1"); err != nil {
		return err
	}

	// Create compound index on title_id and created_at (for query optimization)
	titleCreatedIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "title_id", Value: 1}, {Key: "created_at", Value: 1}},
		Options: options.Index().
			SetName("title_id_1_created_at_1"),
	}
	if err := createIndexIfNotExists(ctx, coll, titleCreatedIndex, "title_id_1_created_at_1"); err != nil {
		return err
	}

	return nil
}

// createIndexIfNotExists checks if an index exists and creates it if it doesn't
func createIndexIfNotExists(ctx context.Context, coll *mongo.Collection, indexModel mongo.IndexModel, indexName string) error {
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
		fmt.Printf("ℹ️  Index '%s' already exists on collection '%s', skipping...\n", indexName, coll.Name())
		return nil
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
