// Package config exposes runtime-tunable settings sourced from environment
// variables, with safe defaults. Values are read on demand (cheap) so they stay
// testable and require no explicit load step.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Pagination defaults (used when the corresponding env var is unset/invalid).
const (
	defaultPageSize    = 20
	defaultMaxPageSize = 100
	defaultSearchLimit = 5
)

// DefaultPageSize is the page size used when a request omits/zeroes it.
// Override with DEFAULT_PAGE_SIZE.
func DefaultPageSize() int { return envInt("DEFAULT_PAGE_SIZE", defaultPageSize) }

// MaxPageSize caps the page size a client may request. Override with MAX_PAGE_SIZE.
func MaxPageSize() int { return envInt("MAX_PAGE_SIZE", defaultMaxPageSize) }

// DefaultSearchLimit is the title-search result cap when unspecified.
// Override with DEFAULT_SEARCH_LIMIT.
func DefaultSearchLimit() int { return envInt("DEFAULT_SEARCH_LIMIT", defaultSearchLimit) }

// envInt returns a positive integer from the named env var, or def when the var
// is unset, blank, non-numeric, or non-positive.
func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
