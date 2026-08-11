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

// ratingRowToModel assembles a database.Rating row plus its (possibly nil)
// season map into the storage-neutral models.UserRating.
func ratingRowToModel(r database.Rating, seasons *models.SeasonsRatings) models.UserRating {
	return models.UserRating{
		Id:             r.ID,
		TitleId:        r.TitleID,
		SeasonsRatings: seasons,
		UserId:         r.UserID,
		GroupId:        r.GroupID,
		Note:           r.Note,
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

// ratingSeasonRowToItem converts a database.RatingSeason row into a
// models.SeasonRatingItem.
func ratingSeasonRowToItem(s database.RatingSeason) models.SeasonRatingItem {
	return models.SeasonRatingItem{
		Rating:    s.Rating,
		AddedAt:   s.AddedAt.Time,
		UpdatedAt: s.UpdatedAt.Time,
	}
}

// assembleSeasonsRatings groups rating_seasons rows for a single rating into
// a *models.SeasonsRatings, matching the store's nil/empty convention: nil
// when there are no season rows (movie ratings), a non-nil map otherwise.
func assembleSeasonsRatings(rows []database.RatingSeason) *models.SeasonsRatings {
	if len(rows) == 0 {
		return nil
	}
	out := make(models.SeasonsRatings, len(rows))
	for _, r := range rows {
		out[r.Season] = ratingSeasonRowToItem(r)
	}
	return &out
}

// commentRowToModel assembles a database.Comment row plus its (possibly nil)
// season map into the storage-neutral models.Comment. Comment is nullable at
// the column level (movies set it, series leave it NULL), mapped via
// textToPtr; seasons is nil for a movie comment and non-nil (the season map)
// for a series comment, matching the store's convention.
func commentRowToModel(c database.Comment, seasons *models.SeasonsComments) models.Comment {
	return models.Comment{
		Id:              c.ID,
		TitleId:         c.TitleID,
		UserId:          c.UserID,
		GroupId:         c.GroupID,
		Comment:         textToPtr(c.Comment),
		SeasonsComments: seasons,
		CreatedAt:       c.CreatedAt.Time,
		UpdatedAt:       c.UpdatedAt.Time,
	}
}

// commentSeasonRowToItem converts a database.CommentSeason row into a
// models.SeasonCommentItem.
func commentSeasonRowToItem(s database.CommentSeason) models.SeasonCommentItem {
	return models.SeasonCommentItem{
		Comment:   s.Comment,
		AddedAt:   s.AddedAt.Time,
		UpdatedAt: s.UpdatedAt.Time,
	}
}

// assembleSeasonsComments groups comment_seasons rows for a single comment
// into a *models.SeasonsComments, matching the store's nil/empty convention:
// nil when there are no season rows (movie comments), a non-nil map
// otherwise (series comments).
func assembleSeasonsComments(rows []database.CommentSeason) *models.SeasonsComments {
	if len(rows) == 0 {
		return nil
	}
	out := make(models.SeasonsComments, len(rows))
	for _, r := range rows {
		out[r.Season] = commentSeasonRowToItem(r)
	}
	return &out
}

// groupRowToModel assembles a database.Group row plus its resolved member ids
// and its already-assembled titles map into the storage-neutral models.Group.
// titles is always a non-nil (possibly empty) map: a group always reports a
// titles map, even when it holds none.
func groupRowToModel(g database.Group, users []string, titles models.GroupTitles) models.Group {
	return models.Group{
		Id:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		OwnerId:     g.OwnerID,
		Users:       users,
		Titles:      titles,
		CreatedAt:   g.CreatedAt.Time,
		UpdatedAt:   g.UpdatedAt.Time,
		Deleted:     g.Deleted,
		DeletedAt:   timestamptzToPtr(g.DeletedAt),
	}
}

// groupTitleRowToModel converts a database.GroupTitle row plus its (possibly
// nil) per-season watched map into a models.GroupTitleItem. seasons is nil for
// a movie (no season rows) and non-nil for a series, matching the store's
// seasonsWatched convention.
func groupTitleRowToModel(t database.GroupTitle, seasons *models.SeasonsWatched) models.GroupTitleItem {
	return models.GroupTitleItem{
		TitleId:        t.TitleID,
		SeasonsWatched: seasons,
		Watched:        t.Watched,
		AddedAt:        t.AddedAt.Time,
		UpdatedAt:      t.UpdatedAt.Time,
		WatchedAt:      timestamptzToPtr(t.WatchedAt),
	}
}

// groupTitleSeasonRowToItem converts a database.GroupTitleSeason row into a
// models.SeasonWatchedItem.
func groupTitleSeasonRowToItem(s database.GroupTitleSeason) models.SeasonWatchedItem {
	return models.SeasonWatchedItem{
		Watched:   s.Watched,
		WatchedAt: timestamptzToPtr(s.WatchedAt),
		AddedAt:   s.AddedAt.Time,
		UpdatedAt: s.UpdatedAt.Time,
	}
}

// assembleSeasonsWatched groups group_title_seasons rows for a single title
// into a *models.SeasonsWatched, matching the store's nil/empty convention: nil
// when there are no season rows (a movie), a non-nil map otherwise (a series).
func assembleSeasonsWatched(rows []database.GroupTitleSeason) *models.SeasonsWatched {
	if len(rows) == 0 {
		return nil
	}
	out := make(models.SeasonsWatched, len(rows))
	for _, r := range rows {
		out[r.Season] = groupTitleSeasonRowToItem(r)
	}
	return &out
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
//
// json.Unmarshal preserves JSON null as a nil Go slice, but a title read must
// always report these 8 slice fields as non-nil (possibly empty) — that is the
// shape the API contract depends on. Normalize them here. Genres is deliberately
// left untouched: it is passed through as stored, nil included.
func rowToTitle(r database.Title) (models.Title, error) {
	var t models.Title
	if err := json.Unmarshal(r.Metadata, &t); err != nil {
		return models.Title{}, fmt.Errorf("unmarshal title metadata: %w", err)
	}

	if t.Directors == nil {
		t.Directors = []models.Person{}
	}
	if t.Writers == nil {
		t.Writers = []models.Person{}
	}
	if t.Stars == nil {
		t.Stars = []models.Person{}
	}
	if t.OriginCountries == nil {
		t.OriginCountries = []models.CodeName{}
	}
	if t.SpokenLanguages == nil {
		t.SpokenLanguages = []models.CodeName{}
	}
	if t.Interests == nil {
		t.Interests = []models.Interest{}
	}
	if t.Seasons == nil {
		t.Seasons = []models.Seasons{}
	}
	if t.Episodes == nil {
		t.Episodes = []models.Episode{}
	}

	return t, nil
}
