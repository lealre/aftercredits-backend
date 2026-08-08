package postgres

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

// schemaDir is the goose migrations directory, relative to this package
// (go test sets the working directory to the package under test).
const schemaDir = "../../sql/schema"

var testPool *pgxpool.Pool

// TestMain starts a single postgres:16 container for the whole package,
// applies the goose migrations once, and hands out a shared *pgxpool.Pool.
// Individual tests call resetDB(t) between runs instead of paying for a
// fresh container each time.
func TestMain(m *testing.M) {
	ctx := context.Background()

	dsn, terminate, err := startPostgresContainer(ctx)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	defer terminate()

	if err := runMigrations(dsn, migrateUp); err != nil {
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

// newTestPool returns the package-wide pgxpool.Pool connected to the
// shared, already-migrated test container.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Fatal("test pool not initialized — is TestMain running?")
	}
	return testPool
}

// newTestStore builds a Store on top of the shared test pool, for tests that
// exercise the postgres.Store methods directly.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(newTestPool(t))
}

// resetDB truncates every table so each test starts from a clean slate,
// mirroring the integration suite's resetDB (tests/setup_test.go).
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

// startPostgresContainer spins up a fresh postgres:16 container and returns
// its DSN plus a cleanup function that terminates it.
func startPostgresContainer(ctx context.Context) (dsn string, terminate func(), err error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", nil, fmt.Errorf("run postgres container: %w", err)
	}

	dsn, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return "", nil, fmt.Errorf("get connection string: %w", err)
	}

	terminate = func() {
		tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = pgContainer.Terminate(tctx)
	}

	return dsn, terminate, nil
}

type migrateDirection string

const (
	migrateUp   migrateDirection = "up"
	migrateDown migrateDirection = "down"
)

// runMigrations opens its own *sql.DB (via the pgx stdlib driver) against
// dsn and runs the goose migrations in schemaDir in the given direction.
func runMigrations(dsn string, direction migrateDirection) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql.DB: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	switch direction {
	case migrateUp:
		return goose.Up(db, schemaDir)
	case migrateDown:
		return goose.Down(db, schemaDir)
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
}

// tableNames lists every table 001_init.sql creates, used to assert the
// schema is fully applied after "goose up".
var tableNames = []string{
	"users", "titles", "ratings", "rating_seasons",
	"comments", "comment_seasons", "groups", "group_members",
	"group_titles", "group_title_seasons",
}

// existingTables returns which of tableNames are currently present in the
// public schema. goose's own bookkeeping table (goose_db_version) is
// excluded — it isn't part of 001_init.sql's schema.
func existingTables(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`)
	if err != nil {
		t.Fatalf("failed to query information_schema.tables: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan table name: %v", err)
		}
		found = append(found, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}
	return found
}

// TestMigrationsUpDown runs goose up -> down -> up against a fresh
// container (independent of the package's shared one) and asserts that:
//   - each step succeeds without error, and
//   - after the final "up", all 10 tables from 001_init.sql exist.
func TestMigrationsUpDown(t *testing.T) {
	ctx := context.Background()

	dsn, terminate, err := startPostgresContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer terminate()

	if err := runMigrations(dsn, migrateUp); err != nil {
		t.Fatalf("goose up failed: %v", err)
	}
	if err := runMigrations(dsn, migrateDown); err != nil {
		t.Fatalf("goose down failed: %v", err)
	}
	if err := runMigrations(dsn, migrateUp); err != nil {
		t.Fatalf("second goose up failed: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open sql.DB: %v", err)
	}
	defer db.Close()

	found := existingTables(t, db)
	for _, want := range tableNames {
		ok := false
		for _, got := range found {
			if got == want {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("expected table %q to exist after migrations, found tables: %v", want, found)
		}
	}
	if len(found) != len(tableNames) {
		t.Errorf("expected exactly %d tables, found %d: %v", len(tableNames), len(found), found)
	}
}
