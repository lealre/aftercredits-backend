package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/server"
)

func main() {
	_ = godotenv.Load()

	pool, err := postgres.Connect(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	if err = server.ListenAndServe(postgres.New(pool)); err != nil {
		log.Fatalf("error while starting server: %v", err)
	}
}
