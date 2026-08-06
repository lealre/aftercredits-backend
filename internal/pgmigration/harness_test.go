package pgmigration

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// schemaDir is the goose migrations directory, relative to this package.
const schemaDir = "../../sql/schema"

var testPool *pgxpool.Pool

// TestMain starts a single postgres:16 container for the whole package,
// applies the goose migrations once, and hands out a shared *pgxpool.Pool,
// mirroring internal/postgres/testharness_test.go.
func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() {
		tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = pgContainer.Terminate(tctx)
	}()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed to get connection string: %v", err)
	}

	if err := runMigrationsUp(dsn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to create pgx pool: %v", err)
	}
	defer pool.Close()
	testPool = pool

	os.Exit(m.Run())
}

// runMigrationsUp applies the goose schema via the pgx stdlib driver.
func runMigrationsUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql.DB: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.Up(db, schemaDir)
}

// newTestPool returns the package-wide pool connected to the shared,
// already-migrated test container.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Fatal("test pool not initialized — is TestMain running?")
	}
	return testPool
}

// resetDB truncates every table so each test starts from a clean slate.
func resetDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	const stmt = `TRUNCATE users, titles, ratings, rating_seasons, comments,
		comment_seasons, groups, group_members, group_titles,
		group_title_seasons RESTART IDENTITY CASCADE`
	if _, err := newTestPool(t).Exec(ctx, stmt); err != nil {
		t.Fatalf("failed to reset db: %v", err)
	}
}
