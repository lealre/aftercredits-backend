package postgres

import (
	"database/sql"
	"fmt"

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

	goose.SetBaseFS(sqlassets.SchemaFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.Up(db, "schema")
}
