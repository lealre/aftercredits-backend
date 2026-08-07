package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/lealre/movies-backend/internal/pgmigration"
)

// One-time Mongo -> Postgres data migration (v0.1.0 sub-project 3): reads an
// extracted mongodump backup, loads it into a goose-migrated Postgres
// preserving every id and timestamp, then verifies by reading everything
// back. See README.md for the runbook.
func main() {
	_ = godotenv.Load()

	dumpDir := flag.String("dump", "", "path to the extracted mongodump directory (required)")
	dbURL := flag.String("db", "", "Postgres URL (default: built from POSTGRES_* env vars)")
	reset := flag.Bool("reset", false, "truncate all target tables before loading")
	flag.Parse()

	if *dumpDir == "" {
		flag.Usage()
		log.Fatal("❌ -dump is required")
	}
	if *dbURL == "" {
		*dbURL = postgresURLFromEnv()
	}

	ctx := context.Background()

	fmt.Printf("📦 Reading dump from %s...\n", *dumpDir)
	dump, err := pgmigration.ReadDump(*dumpDir)
	if err != nil {
		log.Fatalf("❌ Failed to read dump: %v", err)
	}
	for _, w := range dump.Warnings {
		fmt.Printf("⚠️  %s\n", w)
	}
	fmt.Printf("   users=%d titles=%d ratings=%d comments=%d groups=%d\n",
		len(dump.Users), len(dump.Titles), len(dump.Ratings), len(dump.Comments), len(dump.Groups))

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Postgres: %v", err)
	}
	defer pool.Close()

	fmt.Println("🚚 Loading into Postgres (single transaction)...")
	stats, err := pgmigration.Load(ctx, pool, dump, *reset)
	if err != nil {
		log.Fatalf("❌ Load failed (nothing committed): %v", err)
	}
	fmt.Printf("   users=%d titles=%d groups=%d members=%d groupTitles=%d groupTitleSeasons=%d ratings=%d ratingSeasons=%d comments=%d commentSeasons=%d\n",
		stats.Users, stats.Titles, stats.Groups, stats.GroupMembers, stats.GroupTitles,
		stats.GroupTitleSeasons, stats.Ratings, stats.RatingSeasons, stats.Comments, stats.CommentSeasons)

	fmt.Println("🔎 Verifying (full read-back)...")
	res := pgmigration.Verify(ctx, pool, dump)
	for _, w := range res.Warnings {
		fmt.Printf("⚠️  %s\n", w)
	}
	if !res.OK() {
		for _, f := range res.Failures {
			fmt.Printf("❌ %s\n", f)
		}
		fmt.Printf("❌ Verification failed: %d failure(s) across %d checked record(s)\n", len(res.Failures), res.Checked)
		os.Exit(1)
	}
	fmt.Printf("✅ Migration complete: %d record(s) verified, %d warning(s)\n", res.Checked, len(res.Warnings))
}

// postgresURLFromEnv builds the connection URL from POSTGRES_* env vars,
// defaulting to the docker-compose service's credentials (mirrors how
// internal/mongodb builds its URI from MONGO_* vars).
func postgresURLFromEnv() string {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		get("POSTGRES_USER", "aftercredits"),
		get("POSTGRES_PASSWORD", "aftercredits"),
		get("POSTGRES_HOST", "localhost"),
		get("POSTGRES_PORT", "5432"),
		get("POSTGRES_DB", "aftercredits"))
}
