package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	sqlassets "github.com/lealre/movies-backend/sql"
)

// Migrate applies the embedded goose schema migrations, over a pgx-stdlib
// *sql.DB built from the same POSTGRES_* env Connect uses.
//
// goose applies only the versions the database has not seen, so re-running it
// against an up-to-date schema is a no-op. It is invoked by `database -migrate`,
// which the deploy runs as its own step before the server starts — the schema is
// deliberately not changed from inside the running application.
func Migrate() error {
	db, err := sql.Open("pgx", URI())
	if err != nil {
		return fmt.Errorf("open sql.DB: %w", err)
	}
	defer db.Close()

	// The same retry the server does, and for the same reason: postgres is a
	// separate stack, so nothing orders it before this runs. This path matters
	// MORE than the server's — it is the first thing db-setup executes, so it
	// is what actually meets a database that is still starting.
	//
	// sql.Open does not connect; PingContext is what dials.
	ctx := context.Background()
	if err := retryConnect(ctx, connectAttempts, func() error {
		return db.PingContext(ctx)
	}, time.Sleep); err != nil {
		return err
	}

	goose.SetBaseFS(sqlassets.SchemaFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.Up(db, "schema")
}
