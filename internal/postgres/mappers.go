package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// uniqueViolationCode is the Postgres SQLSTATE for unique_violation.
const uniqueViolationCode = "23505"

// userRowToModel converts a database.User row plus its resolved group ids
// into the storage-neutral models.User used by the service layer.
func userRowToModel(u database.User, groups []string) models.User {
	return models.User{
		Id:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		AvatarURL:    textToPtr(u.AvatarUrl),
		Groups:       groups,
		Role:         models.UserRole(u.Role),
		IsActive:     u.IsActive,
		LastLoginAt:  timestamptzToPtr(u.LastLoginAt),
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

// textToPtr converts a nullable pgtype.Text into a *string, nil when unset.
func textToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// ptrToText converts a *string into a nullable pgtype.Text, matching
// textToPtr's nil <-> unset convention.
func ptrToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// timestamptzToPtr converts a nullable pgtype.Timestamptz into a *time.Time,
// nil when unset.
func timestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

// ptrToTimestamptz converts a *time.Time into a nullable pgtype.Timestamptz,
// matching timestamptzToPtr's nil <-> unset convention.
func ptrToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// timeToTimestamptz converts a non-nullable time.Time into an always-valid
// pgtype.Timestamptz.
func timeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// isUniqueViolation reports whether err is a Postgres unique_violation
// (SQLSTATE 23505), e.g. a duplicate username/email.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

// notFound maps pgx.ErrNoRows to store.ErrRecordNotFound, leaving any other
// error unchanged.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrRecordNotFound
	}
	return err
}

// titleToRow converts a models.Title into the params for InsertTitle. The
// query columns (primary_title, type, start_year, ...) are denormalized
// copies used for filtering/sorting/indexing; metadata holds the complete
// serialized title and is the source of truth on read (see rowToTitle).
func titleToRow(t models.Title) (database.InsertTitleParams, error) {
	metadata, err := json.Marshal(t)
	if err != nil {
		return database.InsertTitleParams{}, fmt.Errorf("marshal title metadata: %w", err)
	}
	return database.InsertTitleParams{
		ID:              t.ID,
		PrimaryTitle:    t.PrimaryTitle,
		Type:            t.Type,
		StartYear:       int32(t.StartYear),
		RatingAggregate: t.Rating.AggregateRating,
		VoteCount:       int32(t.Rating.VoteCount),
		AddedAt:         ptrToTimestamptz(t.AddedAt),
		UpdatedAt:       ptrToTimestamptz(t.UpdatedAt),
		Metadata:        metadata,
	}, nil
}

// rowToTitle rebuilds a models.Title from a database.Title row. metadata is
// the source of truth: it holds the complete serialized title, so unmarshaling
// it reproduces the exact models.Title (including nested seasons/episodes/
// cast and nil/empty conventions) that was passed to titleToRow. The
// denormalized query columns on the row are ignored here.
func rowToTitle(r database.Title) (models.Title, error) {
	var t models.Title
	if err := json.Unmarshal(r.Metadata, &t); err != nil {
		return models.Title{}, fmt.Errorf("unmarshal title metadata: %w", err)
	}
	return t, nil
}
