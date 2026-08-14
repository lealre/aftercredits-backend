package tests

import (
	"context"
	"database/sql"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/lealre/movies-backend/internal/database"
	pgstore "github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/server"
)

const schemaDir = "../sql/schema"

var (
	testPool    *pgxpool.Pool
	testStore   *pgstore.Store
	testQueries *database.Queries
	testServer  *httptest.Server
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed to get connection string: %v", err)
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to create pgx pool: %v", err)
	}
	testStore = pgstore.New(testPool)
	testQueries = database.New(testPool)

	// The activity-feed emit sites (Task 5) and routes (Task 6) are inert
	// while this flag is unset. TestMain builds the server once, and
	// t.Setenv is per-test and cannot reach a server already built, so the
	// flag is set here, before the server is constructed, for the whole suite.
	os.Setenv("ACTIVITY_FEED_ENABLED", "true")

	// Bounds the background work the server starts (the activity LISTEN loop),
	// so it and its dedicated connection go away with the suite instead of
	// outliving the pool they are using.
	serverCtx, stopServerWork := context.WithCancel(ctx)

	handler := server.NewServerWithProvider(serverCtx, testStore, newFakeTitleProvider(), "test-secret")
	testServer = httptest.NewServer(handler)

	code := m.Run()

	testServer.Close()
	stopServerWork()
	testPool.Close()
	_ = pgC.Terminate(ctx)

	os.Exit(code)
}

func runMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, schemaDir)
}

func resetDB(t *testing.T) {
	t.Helper()
	const stmt = `TRUNCATE users, titles, ratings, rating_seasons, comments,
		comment_seasons, groups, group_members, group_titles,
		group_title_seasons, activity_events, activity_event_reads
		RESTART IDENTITY CASCADE`
	if _, err := testPool.Exec(context.Background(), stmt); err != nil {
		t.Fatalf("failed to reset db: %v", err)
	}
}
