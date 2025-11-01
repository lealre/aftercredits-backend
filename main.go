package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/server"
)

func main() {
	_ = godotenv.Load()

	db, err := mongodb.Connect(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Disconnect(context.Background())

	if err = server.ListenAndServe(db); err != nil {
		log.Fatalf("error while starting server: %v", err)
	}
}
