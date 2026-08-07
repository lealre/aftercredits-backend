package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/lealre/movies-backend/internal/store"
	sqlassets "github.com/lealre/movies-backend/sql"
)

func main() {
	_ = godotenv.Load()

	migrate := flag.Bool("migrate", false, "apply the embedded goose schema migrations")
	superuser := flag.Bool("superuser", false, "create a superuser if it does not exist")
	flag.Parse()

	ctx := context.Background()

	switch {
	case *migrate:
		if err := runMigrations(); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		fmt.Println("✅ Migrations applied successfully!")

	case *superuser:
		pool, err := postgres.Connect(ctx)
		if err != nil {
			log.Fatalf("Failed to connect to Postgres: %v", err)
		}
		defer pool.Close()
		if err := createSuperuser(ctx, postgres.New(pool)); err != nil {
			log.Fatalf("Failed to create superuser: %v", err)
		}
		fmt.Println("✅ Superuser command ran successfully!")

	default:
		fmt.Println("No valid command specified.")
		flag.Usage()
	}
}

// runMigrations applies the embedded goose migrations over a pgx-stdlib
// *sql.DB built from the same POSTGRES_* env postgres.Connect uses.
func runMigrations() error {
	db, err := sql.Open("pgx", postgres.URI())
	if err != nil {
		return fmt.Errorf("open sql.DB: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(sqlassets.SchemaFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.Up(db, "schema")
}

func createSuperuser(ctx context.Context, db store.Store) error {
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
	if !errors.Is(err, store.ErrRecordNotFound) {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user with Admin role
	now := time.Now()
	userDb := models.User{
		Id:           uuid.NewString(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         models.RoleAdmin,
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
