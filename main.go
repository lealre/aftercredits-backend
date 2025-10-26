package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/server"
)

func main() {
	godotenv.Load()

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
