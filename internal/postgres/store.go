package postgres

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/store"
)

// Store implements store.Store against PostgreSQL, wrapping a *pgxpool.Pool
// and the generated sqlc queries.
type Store struct {
	pool *pgxpool.Pool
	q    *database.Queries
}

var _ store.Store = (*Store)(nil)

// New builds a Store from an already-connected pgxpool.Pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: database.New(pool)}
}

// pageOffset computes the SQL OFFSET for a 1-based page of the given size —
// the LIMIT/OFFSET arithmetic every paged read in this package shares
// (GetTitlesPage, GetGroupTitlesPage). It is one function on purpose: the two
// callers feed the two highest-traffic read endpoints, and a copy per caller
// is a copy to keep in sync by hand.
//
// ok is false when the request can never select a row, and the caller should
// return its empty page (with the correct total) rather than query — never an
// error. Two ways that happens:
//
//   - size <= 0. A non-positive LIMIT selects nothing. size is caller-supplied
//     and unvalidated in the store (clamping is the service's job) and these
//     methods are reachable directly from tests and internal tools, so it has
//     to be handled here — not least because the division below would panic on
//     size == 0 and take the process down (there is no panic-recovery
//     middleware).
//   - the offset would overflow int64. The service bounds size but not page,
//     so an extreme page can make (page-1)*size wrap before the product is
//     ever compared against anything; the guard must therefore run BEFORE any
//     multiplication, not after computing the product. (page-1) cannot
//     overflow on its own (page >= 1 after service normalization), and
//     comparing it against the floor division math.MaxInt64/size predicts
//     whether the product would exceed MaxInt64 without computing it: for
//     positive integers a, b, N, a > N/b (floor) implies a*b > N. Once that
//     guard is false the multiply is provably in int64 range — and int64 is
//     exactly what the generated PageOffset params take (page_offset is cast
//     to bigint in both queries precisely so nothing here has to narrow).
func pageOffset(size, page int) (int64, bool) {
	if size <= 0 {
		return 0, false
	}
	if int64(page-1) > math.MaxInt64/int64(size) {
		return 0, false
	}
	return int64(page-1) * int64(size), true // now provably in range
}

// inTx runs fn inside its own transaction, rolling back on error and
// committing on success. It exists for the writes that genuinely span more
// than one statement (a rating and its season rows, a group and its members);
// a single-statement write needs no transaction, since Postgres commits it on
// its own.
func (s *Store) inTx(ctx context.Context, fn func(q *database.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
