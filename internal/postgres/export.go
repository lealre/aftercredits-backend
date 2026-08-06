package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
)

// Exported helpers for the one-time Mongo -> Postgres data migration
// (internal/pgmigration), which inserts rows through the sqlc queries
// directly and must build params exactly the way the store does. Pure
// delegation so the conventions stay defined in one place.

func TitleToRow(t models.Title) (database.InsertTitleParams, error) { return titleToRow(t) }

func TimeToTimestamptz(t time.Time) pgtype.Timestamptz { return timeToTimestamptz(t) }

func PtrToTimestamptz(t *time.Time) pgtype.Timestamptz { return ptrToTimestamptz(t) }

func PtrToText(s *string) pgtype.Text { return ptrToText(s) }
