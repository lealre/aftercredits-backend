package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
)

// Standalone entrypoint for the same backfill the `database -backfill-groups`
// command runs (and that db-setup runs automatically on deploy). Kept for
// running the backfill by hand against an arbitrary environment.
func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)

	fmt.Println("🔄 Backfilling legacy group fields (deleted, description)...")
	deletedN, descN, err := db.BackfillGroupFields(ctx)
	if err != nil {
		log.Fatalf("Failed to backfill: %v", err)
	}
	fmt.Printf("✅ Backfilled deleted=false on %d group(s), description=\"\" on %d group(s)\n", deletedN, descN)
	fmt.Println("⚠️  Now rebuild the group unique index (reset) so the new partial filter applies.")
}
