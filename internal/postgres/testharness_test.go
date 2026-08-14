package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
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
		group_title_seasons, activity_events, activity_event_reads, activity_read_floors
		RESTART IDENTITY CASCADE`

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

// tableNames lists every relation the migrations leave behind, used to assert
// the schema is fully applied after "goose up". Views count: information_schema
// .tables lists them alongside tables, and activity_visible_events is schema the
// activity queries depend on exactly as much as a table.
var tableNames = []string{
	"users", "titles", "ratings", "rating_seasons",
	"comments", "comment_seasons", "groups", "group_members",
	"group_titles", "group_title_seasons",
	"activity_events", "activity_event_reads", "activity_read_floors", "activity_visible_events",
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

// seedPreGroupScopedRow inserts a group (optionally soft-deleted) that holds
// titleId, optionally with userId as a member, plus — when rowId is non-empty —
// a pre-003 rating and comment by userId on titleId. The ratings/comments rows
// are written without group_id, which is exactly the shape migration 003 has to
// attribute.
func seedPreGroupScopedRow(t *testing.T, db *sql.DB, groupId, userId, titleId string, deleted, isMember bool) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO groups (id, name, owner_id, deleted) VALUES ($1, $1, $2, $3)`,
		groupId, userId, deleted); err != nil {
		t.Fatalf("failed to seed group %s: %v", groupId, err)
	}
	if isMember {
		if _, err := db.Exec(`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, groupId, userId); err != nil {
			t.Fatalf("failed to seed membership of %s in %s: %v", userId, groupId, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO group_titles (group_id, title_id) VALUES ($1, $2)`, groupId, titleId); err != nil {
		t.Fatalf("failed to seed group title %s in %s: %v", titleId, groupId, err)
	}
}

// seedPreGroupScopedFacts inserts one rating and one comment by userId on
// titleId, in the pre-003 shape (no group_id column to fill).
func seedPreGroupScopedFacts(t *testing.T, db *sql.DB, rowId, userId, titleId string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO ratings (id, title_id, user_id, note) VALUES ($1, $2, $3, 7)`,
		rowId, titleId, userId); err != nil {
		t.Fatalf("failed to seed rating %s: %v", rowId, err)
	}
	if _, err := db.Exec(`INSERT INTO comments (id, title_id, user_id, comment) VALUES ($1, $2, $3, 'seeded')`,
		rowId, titleId, userId); err != nil {
		t.Fatalf("failed to seed comment %s: %v", rowId, err)
	}
}

// truncatePreGroupScoped clears everything migration 003 reads, so the next
// case starts from a clean slate on the same container.
func truncatePreGroupScoped(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`TRUNCATE ratings, rating_seasons, comments, comment_seasons,
		groups, group_members, group_titles, group_title_seasons CASCADE`); err != nil {
		t.Fatalf("failed to truncate between migration cases: %v", err)
	}
}

// TestMigration003BackfillsGroupId exercises migration 003 against seeded
// pre-003 data, which TestMigrationsUpDown cannot: run against an empty
// database, both guards see NULL and the backfill UPDATEs touch zero rows, so
// the whole block passes vacuously.
//
// All three cases share one container. The two failing cases run first: goose
// wraps the migration in a transaction, so a raise leaves the database at
// version 2 with the columns never added — which is also what makes the final
// success case proof that a failed migration stays re-runnable.
func TestMigration003BackfillsGroupId(t *testing.T) {
	ctx := context.Background()

	dsn, terminate, err := startPostgresContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer terminate()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open sql.DB: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.UpTo(db, schemaDir, 2); err != nil {
		t.Fatalf("goose up to version 2 failed: %v", err)
	}

	assertStoppedAtVersion2 := func(t *testing.T) {
		t.Helper()

		version, err := goose.GetDBVersion(db)
		if err != nil {
			t.Fatalf("failed to read goose version: %v", err)
		}
		if version != 2 {
			t.Errorf("expected the failed migration to record no version, database is at %d", version)
		}

		var hasColumn bool
		if err := db.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'ratings' AND column_name = 'group_id')`).Scan(&hasColumn); err != nil {
			t.Fatalf("failed to check for ratings.group_id: %v", err)
		}
		if hasColumn {
			t.Error("expected the raise to roll back the ADD COLUMN as well, ratings.group_id still exists")
		}
	}

	t.Run("aborts when a row maps to more than one group", func(t *testing.T) {
		truncatePreGroupScoped(t, db)

		// The author is a member of both groups and both hold the title, so
		// nothing but a human decision can say which group owns the rating.
		seedPreGroupScopedRow(t, db, "g-ambiguous-a", "u1", "t1", false, true)
		seedPreGroupScopedRow(t, db, "g-ambiguous-b", "u1", "t1", false, true)
		seedPreGroupScopedFacts(t, db, "r-ambiguous", "u1", "t1")

		err := goose.Up(db, schemaDir)
		if err == nil {
			t.Fatal("expected goose up to fail rather than pick one of the two candidate groups")
		}
		for _, want := range []string{"more than one group", "user u1", "title t1"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected the failure to mention %q, got: %v", want, err)
			}
		}
		assertStoppedAtVersion2(t)
	})

	t.Run("aborts when a row maps to no group", func(t *testing.T) {
		truncatePreGroupScoped(t, db)

		// The group holds the title but the author is not a member — the state
		// LeaveGroup leaves behind.
		seedPreGroupScopedRow(t, db, "g-orphan", "u2", "t2", false, false)
		seedPreGroupScopedFacts(t, db, "r-orphan", "u2", "t2")

		err := goose.Up(db, schemaDir)
		if err == nil {
			t.Fatal("expected goose up to fail rather than leave an unattributable row")
		}
		for _, want := range []string{"no group", "user u2", "title t2"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected the failure to mention %q, got: %v", want, err)
			}
		}
		assertStoppedAtVersion2(t)
	})

	t.Run("backfills the single group that holds the title and has the author as a member", func(t *testing.T) {
		truncatePreGroupScoped(t, db)

		// Only g-owner qualifies. g-not-member holds the title but the author is
		// not in it; g-deleted has both but is soft-deleted. Either predicate
		// going missing makes the row ambiguous and aborts the migration.
		seedPreGroupScopedRow(t, db, "g-owner", "u3", "t3", false, true)
		seedPreGroupScopedRow(t, db, "g-not-member", "u3", "t3", false, false)
		seedPreGroupScopedRow(t, db, "g-deleted", "u3", "t3", true, true)
		seedPreGroupScopedFacts(t, db, "r-owned", "u3", "t3")

		if err := goose.Up(db, schemaDir); err != nil {
			t.Fatalf("goose up failed on unambiguous data: %v", err)
		}

		var ratingGroup, commentGroup string
		if err := db.QueryRow(`SELECT group_id FROM ratings WHERE id = 'r-owned'`).Scan(&ratingGroup); err != nil {
			t.Fatalf("failed to read the backfilled rating: %v", err)
		}
		if err := db.QueryRow(`SELECT group_id FROM comments WHERE id = 'r-owned'`).Scan(&commentGroup); err != nil {
			t.Fatalf("failed to read the backfilled comment: %v", err)
		}
		if ratingGroup != "g-owner" {
			t.Errorf("expected the rating to be attributed to g-owner, got %q", ratingGroup)
		}
		if commentGroup != "g-owner" {
			t.Errorf("expected the comment to be attributed to g-owner, got %q", commentGroup)
		}

		var rowCount int
		if err := db.QueryRow(`SELECT count(*) FROM ratings`).Scan(&rowCount); err != nil {
			t.Fatalf("failed to count ratings: %v", err)
		}
		if rowCount != 1 {
			t.Errorf("expected the backfill to preserve the single rating, found %d", rowCount)
		}
	})
}
