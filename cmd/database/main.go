package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/lealre/movies-backend/internal/store"
)

func main() {
	_ = godotenv.Load()

	migrate := flag.Bool("migrate", false, "apply the embedded goose schema migrations")
	superuser := flag.Bool("superuser", false, "create a superuser if it does not exist")
	flag.Parse()

	ctx := context.Background()

	switch {
	case *migrate:
		if err := postgres.Migrate(); err != nil {
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

func createSuperuser(ctx context.Context, db store.Store) error {
	username := strings.TrimSpace(os.Getenv("SUPERUSER_USERNAME"))
	email := strings.TrimSpace(os.Getenv("SUPERUSER_EMAIL"))
	password := os.Getenv("SUPERUSER_PASSWORD")

	// No defaults. The previous admin/admin fallback created a live superuser
	// with a guessable password whenever these were unset, which is exactly the
	// account a scanner tries first. An empty variable is now a hard error, and
	// `required: true` on the deploy's env_file makes a missing .env fail the
	// deploy rather than silently defaulting through it.
	if username == "" || password == "" {
		return fmt.Errorf("SUPERUSER_USERNAME and SUPERUSER_PASSWORD must be set; there is no default")
	}

	if len(username) < 3 {
		return fmt.Errorf("username must have at least 3 characters")
	}
	if !users.IsValidUsername(username) {
		return fmt.Errorf("username must contain just letters, numbers, '-' or '_'")
	}

	// Validate email if provided
	if email != "" && !users.IsValidEmail(email) {
		return fmt.Errorf("email format is not valid")
	}

	// Password floor for a superuser is higher than an ordinary account's:
	// this credential can read the whole user directory and delete titles
	// across every group.
	if len(password) < 16 {
		return fmt.Errorf("superuser password must have at least 16 characters")
	}

	// Idempotence keys on "does ANY admin exist", not on this username. That
	// way the provisioning step, which the deploy runs on every boot, is a
	// no-op whenever a real admin is present — so it can never re-mint a
	// superuser after one has been deleted for being compromised. It creates
	// one only when there are zero admins, using the env-supplied credentials.
	hasAdmin, err := db.AdminExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for an existing admin: %w", err)
	}
	if hasAdmin {
		fmt.Println("ℹ️  An admin account already exists, skipping superuser creation")
		return nil
	}

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
