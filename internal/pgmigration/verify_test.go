package pgmigration

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_CleanAfterLoad(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)
	dump := loadFixtureViaFiles(t)

	_, err := Load(ctx, pool, dump, false)
	require.NoError(t, err)

	res := Verify(ctx, pool, dump)
	assert.Empty(t, res.Failures, "failures: %v", res.Failures)
	assert.Empty(t, res.Warnings, "the fixture is self-consistent; warnings: %v", res.Warnings)
	assert.True(t, res.OK())
	// 2 users + 2 titles + 2 ratings + 2 comments + 2 groups
	assert.Equal(t, 10, res.Checked)
}

func TestVerify_DetectsCorruption(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)
	dump := loadFixtureViaFiles(t)

	_, err := Load(ctx, pool, dump, false)
	require.NoError(t, err)

	t.Run("mutated rating note", func(t *testing.T) {
		_, err := pool.Exec(ctx, "UPDATE ratings SET note = note + 1 WHERE id = $1", fixR1)
		require.NoError(t, err)

		res := Verify(ctx, pool, dump)
		assert.False(t, res.OK())
		assert.Contains(t, strings.Join(res.Failures, "\n"), fixR1)

		_, err = pool.Exec(ctx, "UPDATE ratings SET note = note - 1 WHERE id = $1", fixR1)
		require.NoError(t, err)
	})

	t.Run("deleted season row breaks counts and read-back", func(t *testing.T) {
		_, err := pool.Exec(ctx, "DELETE FROM group_title_seasons WHERE group_id = $1 AND season = '2'", fixG1)
		require.NoError(t, err)

		res := Verify(ctx, pool, dump)
		assert.False(t, res.OK())
		joined := strings.Join(res.Failures, "\n")
		assert.Contains(t, joined, "group_title_seasons")
	})
}

func TestVerify_MembershipDriftWarns(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)

	dump := fixtureDump()
	// Mongo drift: the user document claims a group the group side doesn't
	// know about. group_members is built from group.users[], so the load is
	// fine — the verifier must warn, not fail.
	dump.Users[1].Groups = append(dump.Users[1].Groups, "64b0000000000000000000ff")
	dir := t.TempDir()
	writeDumpFiles(t, dir, dump)
	fromFiles, err := ReadDump(dir)
	require.NoError(t, err)

	_, err = Load(ctx, pool, fromFiles, false)
	require.NoError(t, err)

	res := Verify(ctx, pool, fromFiles)
	assert.True(t, res.OK(), "drift must not fail verification; failures: %v", res.Failures)
	require.NotEmpty(t, res.Warnings)
	assert.Contains(t, strings.Join(res.Warnings, "\n"), fixU2)
}
