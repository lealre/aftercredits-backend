package postgres

import (
	"errors"
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
