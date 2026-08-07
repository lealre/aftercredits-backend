package postgres

import (
	"context"
	"fmt"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

func (s *Store) GetTitleById(ctx context.Context, id string) (models.Title, error) {
	row, err := s.q.GetTitleById(ctx, id)
	if err != nil {
		return models.Title{}, notFound(err)
	}
	return rowToTitle(row)
}

// AddTitle inserts a title. It takes a storage-neutral models.Title and maps
// it to the hybrid JSONB row internally (see titleToRow) before persisting.
func (s *Store) AddTitle(ctx context.Context, title models.Title) error {
	params, err := titleToRow(title)
	if err != nil {
		return err
	}
	if err := s.q.InsertTitle(ctx, params); err != nil {
		if isUniqueViolation(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	return nil
}

func (s *Store) DeleteTitle(ctx context.Context, id string) (bool, error) {
	n, err := s.q.DeleteTitle(ctx, id)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) TitleExists(ctx context.Context, id string) (bool, error) {
	return s.q.TitleExists(ctx, id)
}

// GetTitleTypes fetches title types for the given title IDs, returning a map
// of titleId -> type. Matches the previous behavior: an empty/nil input
// short-circuits to an empty map without hitting the database.
func (s *Store) GetTitleTypes(ctx context.Context, titleIds []string) (map[string]string, error) {
	if len(titleIds) == 0 {
		return make(map[string]string), nil
	}
	rows, err := s.q.GetTitleTypes(ctx, titleIds)
	if err != nil {
		return nil, err
	}
	titleTypes := make(map[string]string, len(rows))
	for _, r := range rows {
		titleTypes[r.ID] = r.Type
	}
	return titleTypes, nil
}

// titleOrderColumns is the FIXED whitelist of orderBy -> titles column used
// by GetTitlesPage's CASE 2 (standard column sort). Column names in the
// generated SQL come ONLY from this map — the caller-supplied orderBy string
// is never interpolated directly. Applies the remapping
// ("" and "imdbRating"), plus every other titles column a client can
// reasonably ask to sort by. Any orderBy value not present here (including
// "watched", which has no titles-table equivalent and only makes sense for
// CASE 1) falls back to the same column as "" (primary_title).
var titleOrderColumns = map[string]string{
	"":             "primary_title",
	"primaryTitle": "primary_title",
	"imdbRating":   "rating_aggregate",
	"startYear":    "start_year",
	"type":         "type",
	"voteCount":    "vote_count",
	"addedAt":      "added_at",
	"updatedAt":    "updated_at",
}

// isGroupFieldOrderBy reports whether orderBy refers to a group-titles-join
// field (watched/watchedAt/addedAt) rather than a native titles column.
// Matches the group-fields sort check.
func isGroupFieldOrderBy(orderBy string) bool {
	return orderBy == "watched" || orderBy == "watchedAt" || orderBy == "addedAt"
}

// GetTitlesPage pages and sorts titles against the
// hybrid JSONB titles table:
//
//   - empty (non-nil) ids -> ([]models.Title{}, 0, nil), no query issued.
//   - WHERE id = ANY(ids) when ids is non-empty, no filter otherwise.
//   - total is COUNT(*) under the same WHERE.
//   - CASE 1: ids non-empty AND orderBy is a group field (watched/watchedAt/
//     addedAt) -> ORDER BY array_position(ids, id), preserving the caller's
//     order.
//   - CASE 2: otherwise -> ORDER BY <whitelisted column> <ASC|DESC>, column
//     from titleOrderColumns, direction from ascending (default ASC).
//   - LIMIT size OFFSET (page-1)*size, matching the previous skip/limit
//     computation verbatim (no extra clamping here; that's the service
//     layer's job).
func (s *Store) GetTitlesPage(
	ctx context.Context,
	ids []string,
	orderBy string,
	ascending *bool,
	size, page int,
) ([]models.Title, int64, error) {
	if ids != nil && len(ids) == 0 {
		return []models.Title{}, 0, nil
	}

	var whereArgs []any
	where := ""
	if len(ids) > 0 {
		whereArgs = append(whereArgs, ids)
		where = "WHERE id = ANY($1::text[])"
	}

	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM titles "+where, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	direction := "ASC"
	if ascending != nil && !*ascending {
		direction = "DESC"
	}

	var orderClause string
	if len(ids) > 0 && isGroupFieldOrderBy(orderBy) {
		orderClause = "ORDER BY array_position($1::text[], id)"
	} else {
		column, ok := titleOrderColumns[orderBy]
		if !ok {
			column = titleOrderColumns[""]
		}
		orderClause = fmt.Sprintf("ORDER BY %s %s", column, direction)
	}

	args := append([]any{}, whereArgs...)
	args = append(args, size, (page-1)*size)
	limitIdx := len(whereArgs) + 1
	offsetIdx := len(whereArgs) + 2

	query := fmt.Sprintf(
		`SELECT id, primary_title, type, start_year, rating_aggregate, vote_count, added_at, updated_at, metadata
		 FROM titles %s %s LIMIT $%d OFFSET $%d`,
		where, orderClause, limitIdx, offsetIdx,
	)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	titles := []models.Title{}
	for rows.Next() {
		var row database.Title
		if err := rows.Scan(
			&row.ID,
			&row.PrimaryTitle,
			&row.Type,
			&row.StartYear,
			&row.RatingAggregate,
			&row.VoteCount,
			&row.AddedAt,
			&row.UpdatedAt,
			&row.Metadata,
		); err != nil {
			return nil, 0, err
		}
		title, err := rowToTitle(row)
		if err != nil {
			return nil, 0, err
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return titles, total, nil
}

// ListTitleIds returns every title id, ordered. Not part of store.Store —
// used by internal tools (cmd/routines).
func (s *Store) ListTitleIds(ctx context.Context) ([]string, error) {
	ids, err := s.q.ListTitleIds(ctx)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// UpdateTitle rewrites a title row — denormalized query columns and the full
// JSONB metadata — from the given model, preserving the id. Not part of
// store.Store — used by internal tools (cmd/routines). Returns
// store.ErrRecordNotFound if the id does not exist.
func (s *Store) UpdateTitle(ctx context.Context, title models.Title) error {
	params, err := titleToRow(title)
	if err != nil {
		return err
	}
	n, err := s.q.UpdateTitle(ctx, database.UpdateTitleParams{
		ID:              params.ID,
		PrimaryTitle:    params.PrimaryTitle,
		Type:            params.Type,
		StartYear:       params.StartYear,
		RatingAggregate: params.RatingAggregate,
		VoteCount:       params.VoteCount,
		AddedAt:         params.AddedAt,
		UpdatedAt:       params.UpdatedAt,
		Metadata:        params.Metadata,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}
