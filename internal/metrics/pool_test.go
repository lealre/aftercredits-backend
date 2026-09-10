package metrics_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/metrics"
)

func TestNew_ExposesPoolSeriesFromSnapshot(t *testing.T) {
	m := metrics.New(func() metrics.PoolSnapshot {
		return metrics.PoolSnapshot{
			MaxConns:          10,
			TotalConns:        6,
			AcquiredConns:     4,
			IdleConns:         2,
			ConstructingConns: 1,
			Acquires:          120,
			EmptyAcquires:     7,
			CanceledAcquires:  3,
			NewConns:          9,
			AcquireDuration:   2500 * time.Millisecond,
		}
	})

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	// One series per connection state, all under the same metric name.
	for _, want := range []string{
		`aftercredits_db_pool_connections{state="max"} 10`,
		`aftercredits_db_pool_connections{state="total"} 6`,
		`aftercredits_db_pool_connections{state="acquired"} 4`,
		`aftercredits_db_pool_connections{state="idle"} 2`,
		`aftercredits_db_pool_connections{state="constructing"} 1`,
	} {
		require.Contains(t, body, want, "exposition should carry %s", want)
	}

	// The cumulative counters. empty_ and canceled_ are the saturation
	// signals; acquire_duration is rendered in seconds from a Duration.
	for _, want := range []string{
		"aftercredits_db_pool_acquires_total 120",
		"aftercredits_db_pool_empty_acquires_total 7",
		"aftercredits_db_pool_canceled_acquires_total 3",
		"aftercredits_db_pool_new_connections_total 9",
		"aftercredits_db_pool_acquire_duration_seconds_total 2.5",
	} {
		require.Contains(t, body, want, "exposition should carry %s", want)
	}
}

// The snapshot is read per scrape, not captured at construction, so each
// scrape reflects the pool's current state.
func TestPool_StatsAreReadPerScrape(t *testing.T) {
	idle := int32(1)
	m := metrics.New(func() metrics.PoolSnapshot {
		return metrics.PoolSnapshot{IdleConns: idle}
	})

	read := func() string {
		rec := httptest.NewRecorder()
		m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
		return rec.Body.String()
	}

	require.Contains(t, read(), `aftercredits_db_pool_connections{state="idle"} 1`)

	idle = 5
	require.Contains(t, read(), `aftercredits_db_pool_connections{state="idle"} 5`,
		"a second scrape must reflect the new snapshot")
}
