package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool sizing. The pgx default is max(4, NumCPU), i.e. 4 on the Pi, which a
// couple of hundred concurrent requests can exhaust — queuing everyone,
// including login, behind a full pool. These are overridable via env for other
// environments.
const (
	defaultMaxConns        = 16
	defaultMinConns        = 2
	defaultMaxConnLifetime = time.Hour
	defaultMaxConnIdleTime = 30 * time.Minute
)

// Connect builds a pgxpool.Pool from the POSTGRES_* environment variables and
// pings before returning so a bad URL fails at startup, not on the first query.
// The pool is sized explicitly (see the constants above).
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	if os.Getenv("POSTGRES_PASSWORD") == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD must be set; there is no default")
	}

	cfg, err := pgxpool.ParseConfig(URI())
	if err != nil {
		return nil, fmt.Errorf("postgres config error: %w", err)
	}

	cfg.MaxConns = clampToInt32(envInt("POSTGRES_MAX_CONNS", defaultMaxConns))
	cfg.MinConns = clampToInt32(envInt("POSTGRES_MIN_CONNS", defaultMinConns))
	cfg.MaxConnLifetime = defaultMaxConnLifetime
	cfg.MaxConnIdleTime = defaultMaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres pool error: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping error: %w", err)
	}
	return pool, nil
}

// URI returns the POSTGRES_*-derived connection string Connect uses.
//
// POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB have NO defaults: the old
// fallback to the literal "aftercredits" meant a deploy with a missing or
// blank password silently connected with a credential published in a public
// repo. A blank one now yields an unusable URI that fails the ping loudly at
// startup — the same fail-fast contract as the JWT secret. Host and port keep
// convenience defaults; they are connection locators, not secrets.
func URI() string {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		get("POSTGRES_HOST", "localhost"),
		get("POSTGRES_PORT", "5432"),
		os.Getenv("POSTGRES_DB"))
}

// envInt returns a positive int from the named env var, or def otherwise.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}
