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
