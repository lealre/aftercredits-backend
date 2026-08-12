package postgres

import (
	"context"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

func (s *Store) GetTitleById(ctx context.Context, id string) (models.Title, error) {
	row, err := s.qq(ctx).GetTitleById(ctx, id)
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
	if err := s.qq(ctx).InsertTitle(ctx, params); err != nil {
		if isUniqueViolation(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	return nil
}

func (s *Store) DeleteTitle(ctx context.Context, id string) (bool, error) {
	n, err := s.qq(ctx).DeleteTitle(ctx, id)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) TitleExists(ctx context.Context, id string) (bool, error) {
	return s.qq(ctx).TitleExists(ctx, id)
}

// titleOrderKeys is the sort-key whitelist for GetTitlesPage. Unknown keys
// normalize to "" (primary_title), keeping the requested direction — the
// same fallback GetGroupTitlesPage applies. The actual column mapping now
// lives entirely in the CASE-paired static ORDER BY in sql/queries/titles.sql
// (GetTitlesPage); the caller-supplied orderBy string is never interpolated
// directly into SQL.
var titleOrderKeys = map[string]bool{
	"": true, "primaryTitle": true, "imdbRating": true, "startYear": true,
	"type": true, "voteCount": true, "addedAt": true, "updatedAt": true,
}

// GetTitlesPage pages and sorts titles against the
// hybrid JSONB titles table:
//
//   - total is COUNT(*) over the whole table (CountTitles), fetched
//     unconditionally, same shape as before.
//   - ORDER BY is static in SQL, CASE-paired per whitelisted sort key with
//     direction from ascending (default ASC — see GetTitlesPage in
//     sql/queries/titles.sql). No NULLS FIRST/LAST is added: Postgres'
//     defaults (NULLS LAST for ASC, NULLS FIRST for DESC) keep deciding where
//     rows with a NULL sort value land, so nothing a client sees moves.
//     added_at and updated_at are the only nullable sort keys
//     (rating_aggregate and vote_count are NOT NULL DEFAULT 0, so a "missing"
//     rating sorts as 0, not as NULL).
//   - Every result ends in a deterministic "id ASC" tie-break, so the order is
//     total and paging cannot repeat or skip a row (CONVENTIONS §6): a
//     non-total order under paging lets Postgres return the same row on two
//     pages while never returning another. id is the primary key — unique
//     and NOT NULL — so appending it makes the order total. The tie-break is
//     pinned to ASC and never flipped with the requested direction: its only
//     job is to make ties deterministic, and a fixed direction keeps the rule
//     simple and identical for every sort key.
//   - LIMIT size OFFSET (page-1)*size, matching the previous skip/limit
//     computation verbatim (no extra clamping here; that's the service
//     layer's job). The offset is computed by the shared pageOffset helper,
//     which also decides when a request can select no row at all (a
//     non-positive size, or an offset past int64) and so must yield an empty
//     page instead of a query.
func (s *Store) GetTitlesPage(
	ctx context.Context,
	orderBy string,
	ascending *bool,
	size, page int,
) ([]models.Title, int64, error) {
	if !titleOrderKeys[orderBy] {
		orderBy = ""
	}
	descending := ascending != nil && !*ascending

	total, err := s.qq(ctx).CountTitles(ctx)
	if err != nil {
		return nil, 0, err
	}

	// A non-positive size or an offset that would overflow int64 selects no
	// row at all (see pageOffset): same fallback as a past-the-end page takes
	// below — an empty page with the correct total, never an error.
	offset, ok := pageOffset(size, page)
	if !ok {
		return []models.Title{}, total, nil
	}

	rows, err := s.qq(ctx).GetTitlesPage(ctx, database.GetTitlesPageParams{
		OrderBy:    orderBy,
		Descending: descending,
		PageSize:   int64(size),
		PageOffset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	titles := make([]models.Title, 0, len(rows))
	for _, row := range rows {
		title, err := rowToTitle(row)
		if err != nil {
			return nil, 0, err
		}
		titles = append(titles, title)
	}

	return titles, total, nil
}

// ListTitleIds returns every title id, ordered. Not part of store.Store —
// used by internal tools (cmd/routines).
func (s *Store) ListTitleIds(ctx context.Context) ([]string, error) {
	ids, err := s.qq(ctx).ListTitleIds(ctx)
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
	n, err := s.qq(ctx).UpdateTitle(ctx, database.UpdateTitleParams{
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
