package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/metrics"
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

	// Translate the driver's pool stats into the storage-neutral snapshot the
	// metrics package takes. The *pgxpool.Stat type is inferred from pool, so
	// this entrypoint names no driver type (CONVENTIONS §2), and the snapshot
	// is read per scrape rather than captured now.
	m := metrics.New(func() metrics.PoolSnapshot {
		s := pool.Stat()
		return metrics.PoolSnapshot{
			MaxConns:          s.MaxConns(),
			TotalConns:        s.TotalConns(),
			AcquiredConns:     s.AcquiredConns(),
			IdleConns:         s.IdleConns(),
			ConstructingConns: s.ConstructingConns(),
			Acquires:          s.AcquireCount(),
			EmptyAcquires:     s.EmptyAcquireCount(),
			CanceledAcquires:  s.CanceledAcquireCount(),
			NewConns:          s.NewConnsCount(),
			AcquireDuration:   s.AcquireDuration(),
		}
	})

	if err = server.ListenAndServe(postgres.New(pool), m); err != nil {
		log.Fatalf("error while starting server: %v", err)
	}
}
