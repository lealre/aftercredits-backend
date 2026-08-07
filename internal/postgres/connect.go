package postgres

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect builds a pgxpool.Pool from the POSTGRES_* environment variables,
// following the env-driven pattern. Defaults match the
// docker-compose postgres service. It pings before returning so a bad URL
// fails at startup, not on the first query.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, URI())
	if err != nil {
		return nil, fmt.Errorf("postgres pool error: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping error: %v", err)
	}
	return pool, nil
}

// URI returns the POSTGRES_*-derived connection string Connect uses.
func URI() string {
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
