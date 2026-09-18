package config

import "testing"

func TestPaginationDefaults(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		t.Setenv("DEFAULT_PAGE_SIZE", "")
		t.Setenv("MAX_PAGE_SIZE", "")
		t.Setenv("DEFAULT_SEARCH_LIMIT", "")
		if DefaultPageSize() != 20 || MaxPageSize() != 100 || DefaultSearchLimit() != 5 {
			t.Fatalf("defaults wrong: %d %d %d", DefaultPageSize(), MaxPageSize(), DefaultSearchLimit())
		}
	})

	t.Run("env overrides", func(t *testing.T) {
		t.Setenv("DEFAULT_PAGE_SIZE", "50")
		t.Setenv("MAX_PAGE_SIZE", "200")
		t.Setenv("DEFAULT_SEARCH_LIMIT", "8")
		if DefaultPageSize() != 50 || MaxPageSize() != 200 || DefaultSearchLimit() != 8 {
			t.Fatalf("overrides not applied: %d %d %d", DefaultPageSize(), MaxPageSize(), DefaultSearchLimit())
		}
	})

	t.Run("invalid/non-positive falls back to default", func(t *testing.T) {
		t.Setenv("DEFAULT_PAGE_SIZE", "abc")
		t.Setenv("MAX_PAGE_SIZE", "-5")
		t.Setenv("DEFAULT_SEARCH_LIMIT", "0")
		if DefaultPageSize() != 20 || MaxPageSize() != 100 || DefaultSearchLimit() != 5 {
			t.Fatalf("invalid values should fall back: %d %d %d", DefaultPageSize(), MaxPageSize(), DefaultSearchLimit())
		}
	})
}

func TestMetricsEnabled(t *testing.T) {
	t.Run("defaults to on when unset", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "")
		if !MetricsEnabled() {
			t.Fatal("metrics should default to on")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "true")
		if !MetricsEnabled() {
			t.Fatal("expected true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "false")
		if MetricsEnabled() {
			t.Fatal("expected false")
		}
	})

	t.Run("one is true", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "1")
		if !MetricsEnabled() {
			t.Fatal("expected true")
		}
	})

	t.Run("zero is false", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "0")
		if MetricsEnabled() {
			t.Fatal("expected false")
		}
	})

	// ParseBool rejects "yes", so an unparseable value keeps the default
	// rather than reading as false. This pins behaviour envBool already has,
	// so a future change to that helper is caught here.
	t.Run("unparseable keeps the default", func(t *testing.T) {
		t.Setenv("METRICS_ENABLED", "yes")
		if !MetricsEnabled() {
			t.Fatal("an unparseable value should keep the default (on), not read as false")
		}
	})
}

func TestMetricsAddr(t *testing.T) {
	t.Run("defaults to :9090", func(t *testing.T) {
		t.Setenv("METRICS_ADDR", "")
		if got := MetricsAddr(); got != ":9090" {
			t.Fatalf("expected :9090, got %q", got)
		}
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv("METRICS_ADDR", ":9191")
		if got := MetricsAddr(); got != ":9191" {
			t.Fatalf("expected :9191, got %q", got)
		}
	})

	t.Run("blank falls back to the default", func(t *testing.T) {
		t.Setenv("METRICS_ADDR", "   ")
		if got := MetricsAddr(); got != ":9090" {
			t.Fatalf("expected :9090, got %q", got)
		}
	})
}
