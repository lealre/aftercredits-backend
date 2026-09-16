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

// pageOffset computes the SQL OFFSET for a 1-based page, shared by every paged
// read here so the arithmetic is not copied per caller.
//
// ok is false when the request can never select a row and the caller should
// return an empty page rather than query — never an error. Two cases:
//
//   - size <= 0 selects nothing, and size == 0 would panic the division below.
//     size is caller-supplied and unvalidated in the store (clamping is the
//     service's job), and these methods are reachable from tests and tools.
//   - the offset would overflow int64. The service bounds size but not page, so
//     the guard must run BEFORE the multiply: comparing (page-1) against the
//     floor division MaxInt64/size predicts whether the product would exceed
//     MaxInt64 without computing it, since for positive a, b, N: a > N/b
//     implies a*b > N. Past that guard the multiply is provably in range.
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
