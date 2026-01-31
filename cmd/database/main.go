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

	indexes := flag.Bool("indexes", false, "create indexes in the database if they do not exist")
	resetIndexes := flag.Bool("reset", false, "Delete the indexes and recreate it")
	deleteIndexes := flag.Bool("delete", false, "Delete the indexes")
	superuser := flag.Bool("superuser", false, "create a superuser if it does not exist")

	flag.Parse()

	switch {
	case *indexes:
		if *deleteIndexes {
			if err := mongodb.DeleteAllIndexes(ctx, database); err != nil {
				log.Fatalf("Failed to delete indexes: %v", err)
			}
			fmt.Println("✅ All indexes deleted successfully!")
			return
		}

		if err := mongodb.CreateAllIndexes(ctx, database, *resetIndexes); err != nil {
			log.Fatalf("Failed to create indexes: %v", err)
		}
		fmt.Println("✅ Indexes command ran successfully!")

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

func createSuperuser(ctx context.Context, db *mongodb.DB) error {
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

	// Validate username if provided
	if len(username) < 3 {
		return fmt.Errorf("username must have at least 3 characters")
	}
	if !users.IsValidUsername(username) {
		return fmt.Errorf("username must contain just letters, numbers, '-' or '_'")
	}

	// Validate email if provided (TODO: Add validation from internal package)
	if email != "" && !users.IsValidEmail(email) {
		return fmt.Errorf("email format is not valid")
	}

	// Validate password (TODO: Add validation from internal package)
	if len(password) < 4 {
		return fmt.Errorf("password must have at least 4 characters")
	}

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
