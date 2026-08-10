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

// titleOrderColumns is the FIXED whitelist of orderBy -> titles column used
// by GetTitlesPage. Column names in the generated SQL come ONLY from this
// map — the caller-supplied orderBy string is never interpolated directly.
// Applies the remapping ("" and "imdbRating"), plus every other titles
// column a client can reasonably ask to sort by. Any orderBy value not
// present here (including "watched", which has no titles-table equivalent)
// falls back to the same column as "" (primary_title).
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

// GetTitlesPage pages and sorts titles against the
// hybrid JSONB titles table:
//
//   - total is COUNT(*) over the whole table.
//   - ORDER BY <whitelisted column> <ASC|DESC>, column from titleOrderColumns,
//     direction from ascending (default ASC).
//   - Every result ends in a deterministic "id ASC" tie-break, so the order is
//     total and paging cannot repeat or skip a row (see the ORDER BY below).
//   - LIMIT size OFFSET (page-1)*size, matching the previous skip/limit
//     computation verbatim (no extra clamping here; that's the service
//     layer's job).
func (s *Store) GetTitlesPage(
	ctx context.Context,
	orderBy string,
	ascending *bool,
	size, page int,
) ([]models.Title, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM titles").Scan(&total); err != nil {
		return nil, 0, err
	}

	direction := "ASC"
	if ascending != nil && !*ascending {
		direction = "DESC"
	}

	column, ok := titleOrderColumns[orderBy]
	if !ok {
		column = titleOrderColumns[""]
	}
	// No NULLS FIRST/LAST is added: Postgres' defaults (NULLS LAST for ASC,
	// NULLS FIRST for DESC) keep deciding where rows with a NULL sort value
	// land, so nothing a client sees moves. added_at and updated_at are the
	// only nullable columns in titleOrderColumns (rating_aggregate and
	// vote_count are NOT NULL DEFAULT 0, so a "missing" rating sorts as 0,
	// not as NULL). NULLs tie with one another for sorting, so the appended
	// id only fixes the previously arbitrary order *within* that tie group.
	//
	// The tie-break is pinned to ASC and is never flipped with `direction`:
	// its only job is to make ties deterministic, and a fixed direction keeps
	// the rule simple to state and identical for every sort key. Which rows
	// tie is decided entirely by the primary key, so the visible ordering is
	// unaffected either way. It also feeds LIMIT/OFFSET: a non-total order
	// under paging lets Postgres return the same row on two pages while never
	// returning another (CONVENTIONS §6). id is the primary key — unique and
	// NOT NULL — so appending it makes the order total.
	orderClause := fmt.Sprintf("ORDER BY %s %s, id ASC", column, direction)

	query := fmt.Sprintf(
		`SELECT id, primary_title, type, start_year, rating_aggregate, vote_count, added_at, updated_at, metadata
		 FROM titles %s LIMIT $1 OFFSET $2`,
		orderClause,
	)

	rows, err := s.pool.Query(ctx, query, size, (page-1)*size)
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
