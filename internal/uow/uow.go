// Package uow provides a request-scoped unit of work: one database transaction
// shared by everything a mutating request does, so a business write and the
// activity event describing it commit or roll back together.
//
// The transaction is begun LAZILY, on the first write rather than at the start
// of the request. That is not an optimization: AddTitleToGroup calls an
// external metadata provider before its group write, and beginning eagerly
// would hold a pooled connection open across that network call.
//
// Consequence worth knowing: the transaction spans everything the handler does
// after its first write. No handler today writes, calls out, then writes again.
// One that did would hold a transaction across the network.
package uow

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

type UnitOfWork struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	tx   pgx.Tx
}

func New(pool *pgxpool.Pool) *UnitOfWork { return &UnitOfWork{pool: pool} }

func With(ctx context.Context, u *UnitOfWork) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func FromContext(ctx context.Context) *UnitOfWork {
	u, _ := ctx.Value(ctxKey{}).(*UnitOfWork)
	return u
}

// Tx returns the request's transaction, beginning it on first call.
func (u *UnitOfWork) Tx(ctx context.Context) (pgx.Tx, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.tx != nil {
		return u.tx, nil
	}
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	u.tx = tx
	return tx, nil
}

// Active returns the transaction if one has been begun, without beginning one.
func (u *UnitOfWork) Active() pgx.Tx {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.tx
}

func (u *UnitOfWork) Commit(ctx context.Context) error {
	tx := u.Active()
	if tx == nil {
		return nil
	}
	return tx.Commit(ctx)
}

func (u *UnitOfWork) Rollback(ctx context.Context) error {
	tx := u.Active()
	if tx == nil {
		return nil
	}
	return tx.Rollback(ctx)
}
