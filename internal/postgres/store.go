package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lealre/movies-backend/internal/database"
)

// Store implements store.Store against PostgreSQL, wrapping a *pgxpool.Pool
// and the generated sqlc queries.
type Store struct {
	pool *pgxpool.Pool
	q    *database.Queries
}

// New builds a Store from an already-connected pgxpool.Pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: database.New(pool)}
}
