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

// NormalizePageParams applies the pagination policy the settings above define
// to a caller-supplied size/page: an omitted or non-positive size becomes
// DefaultPageSize(), a size above MaxPageSize() is clamped to it, and a
// non-positive page floors to the first page.
//
// It lives here, next to the two settings it enforces, so every paged endpoint
// normalizes identically — the group-titles and titles listings each used to
// carry their own copy, and they drifted in when the clamping ran relative to
// their empty-result early return. It is pure application policy with nothing
// storage-specific in it, so it stays on the service side of the store
// interface (CONVENTIONS §2).
func NormalizePageParams(size, page int) (int, int) {
	if size <= 0 {
		size = DefaultPageSize()
	}
	if maxSize := MaxPageSize(); size > maxSize {
		size = maxSize
	}
	if page <= 0 {
		page = 1
	}
	return size, page
}

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

// ActivityFeedEnabled reports whether the activity feed is switched on for this
// environment. It defaults to OFF: the feature ships inert, so merging it
// changes nothing in production until it is deliberately enabled.
func ActivityFeedEnabled() bool { return envBool("ACTIVITY_FEED_ENABLED", false) }

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return def
	}
	return v
}
