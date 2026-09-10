package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PoolSnapshot is a storage-neutral reading of a database connection pool at
// one instant.
//
// It carries no driver types on purpose: internal/postgres owns pgx and
// nothing outside it names pgx (CONVENTIONS §2), so the entrypoint translates
// its driver's stats into this shape and this package stays driver-agnostic.
type PoolSnapshot struct {
	MaxConns          int32
	TotalConns        int32
	AcquiredConns     int32
	IdleConns         int32
	ConstructingConns int32

	Acquires         int64
	EmptyAcquires    int64
	CanceledAcquires int64
	NewConns         int64

	// AcquireDuration is the total time spent waiting to acquire a connection.
	AcquireDuration time.Duration
}

// PoolStatsFunc reads the pool's current snapshot. It is called once per
// scrape.
type PoolStatsFunc func() PoolSnapshot

// registerPool adds one series per connection state plus the cumulative
// counters. Implemented in Task 4.
func registerPool(r prometheus.Registerer, stats PoolStatsFunc) {
	_ = r
	_ = stats
}
