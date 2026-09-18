package metrics_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/metrics"
)

// The runtime and process collectors are registered by New, so the exposition
// carries goroutine and memory series without this package instrumenting
// anything. This is the whole reason the "basic stuff" costs no code.
func TestNew_ExposesRuntimeMetrics(t *testing.T) {
	m := metrics.New(nil)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	require.Equal(t, 200, rec.Code, "metrics handler should serve 200")
	body := rec.Body.String()
	for _, want := range []string{"go_goroutines", "go_memstats_heap_inuse_bytes", "process_resident_memory_bytes"} {
		require.Contains(t, body, want, "exposition should carry %s", want)
	}
}

// A nil pool must register no pool series rather than panicking: the database
// CLI and the test helpers have no pool to report on.
func TestNew_NilPoolRegistersNoPoolSeries(t *testing.T) {
	m := metrics.New(nil)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	require.NotContains(t, rec.Body.String(), "aftercredits_db_pool",
		"no pool collector should be registered when pool is nil")
}

// Two registries must not collide. This is why New uses its own registry
// instead of the Prometheus global default — the test suite builds several.
func TestNew_RegistryIsIndependent(t *testing.T) {
	require.NotPanics(t, func() {
		metrics.New(nil)
		metrics.New(nil)
	}, "constructing two registries must not panic on duplicate registration")
}
