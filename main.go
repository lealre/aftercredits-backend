package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/server"
)

func main() {
	// The process default, used by anything logging outside a request: startup,
	// the activity LISTEN loop, and any library reaching for slog.Default().
	// Request-scoped logs get their own logger from the middleware, but they
	// share this handler so the output has one shape.
	slog.SetDefault(slog.New(logx.NewHandler(os.Stdout, logx.LevelFromEnv())))

	_ = godotenv.Load()

	pool, err := postgres.Connect(context.Background())
	if err != nil {
		slog.Error("failed to connect to the database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err = server.ListenAndServe(postgres.New(pool)); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
