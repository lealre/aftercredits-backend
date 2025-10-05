package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	godotenv.Load()

	titleID := "tt0137523"
	body, err := imdb.FetchMovie(titleID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get movie error: %v\n", err)
		os.Exit(1)
	}

	// Decode JSON into a generic map so we can preserve structure
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "json decode error: %v\n", err)
		os.Exit(1)
	}

	// Copy id into _id to use as the MongoDB primary key
	if idVal, ok := doc["id"].(string); ok && idVal != "" {
		doc["_id"] = idVal
	} else {
		fmt.Fprintln(os.Stderr, "missing id in payload; cannot set _id")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use reusable helpers
	if err := mongodb.AddTitle(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			fmt.Println("document already exists; skipping:", doc["_id"])
			return
		}
		fmt.Fprintf(os.Stderr, "add title error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("stored document with _id:", doc["_id"])
}
