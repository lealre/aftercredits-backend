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
// scrape — and scrapes are served concurrently, so it may be called from
// several goroutines at once and the implementation must be safe for
// concurrent use.
type PoolStatsFunc func() PoolSnapshot

// registerPool adds one series per connection state plus the cumulative
// counters.
//
// Gauges and counters are built with NewGaugeFunc/NewCounterFunc rather than a
// hand-written Collector: each series is one declaration, there is no Desc
// bookkeeping to get wrong, and the value is read at scrape time by
// construction.
func registerPool(r prometheus.Registerer, stats PoolStatsFunc) {
	states := []struct {
		state string
		read  func(PoolSnapshot) float64
	}{
		{"max", func(s PoolSnapshot) float64 { return float64(s.MaxConns) }},
		{"total", func(s PoolSnapshot) float64 { return float64(s.TotalConns) }},
		{"acquired", func(s PoolSnapshot) float64 { return float64(s.AcquiredConns) }},
		{"idle", func(s PoolSnapshot) float64 { return float64(s.IdleConns) }},
		{"constructing", func(s PoolSnapshot) float64 { return float64(s.ConstructingConns) }},
	}
	for _, s := range states {
		r.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "db_pool",
			Name:        "connections",
			Help:        "Database pool connections, by state.",
			ConstLabels: prometheus.Labels{"state": s.state},
		}, func() float64 { return s.read(stats()) }))
	}

	counters := []struct {
		name string
		help string
		read func(PoolSnapshot) float64
	}{
		{"acquires_total", "Connections acquired from the pool.", func(s PoolSnapshot) float64 { return float64(s.Acquires) }},
		{"empty_acquires_total", "Acquires that had to wait for a connection to become available.", func(s PoolSnapshot) float64 { return float64(s.EmptyAcquires) }},
		{"canceled_acquires_total", "Acquires abandoned because the waiting context ended.", func(s PoolSnapshot) float64 { return float64(s.CanceledAcquires) }},
		{"new_connections_total", "New connections opened by the pool.", func(s PoolSnapshot) float64 { return float64(s.NewConns) }},
		{"acquire_duration_seconds_total", "Total time spent waiting to acquire a connection.", func(s PoolSnapshot) float64 { return s.AcquireDuration.Seconds() }},
	}
	for _, c := range counters {
		r.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "db_pool",
			Name:      c.name,
			Help:      c.help,
		}, func() float64 { return c.read(stats()) }))
	}
}
