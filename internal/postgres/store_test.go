package postgres

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoDirectQueryUse fails if any store method reaches s.q directly instead of
// going through qq(ctx) or inTx. A direct call runs outside the request's
// unit-of-work transaction, so its write would commit even when the request
// fails — silently defeating the atomicity the unit of work exists to provide.
func TestNoDirectQueryUse(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	allowed := map[string]bool{"store.go": true}
	re := regexp.MustCompile(`\bs\.q\.[A-Z]`)

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || allowed[f] {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		require.NotRegexp(t, re, string(src),
			"%s calls s.q directly; use s.qq(ctx) or s.inTx so the write joins the request transaction", f)
	}
}
