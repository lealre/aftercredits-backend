package pgmigration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/postgres"
)

// loadFixtureViaFiles routes the fixture through the real dump files so
// tests exercise the exact BSON-decoded values production sees (including
// the driver's Local-location times).
func loadFixtureViaFiles(t *testing.T) *Dump {
	t.Helper()
	dir := t.TempDir()
	writeDumpFiles(t, dir, fixtureDump())
	dump, err := ReadDump(dir)
	require.NoError(t, err)
	return dump
}

func TestLoad_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)
	dump := loadFixtureViaFiles(t)

	stats, err := Load(ctx, pool, dump, false)
	require.NoError(t, err)

	assert.Equal(t, LoadStats{
		Users: 2, Titles: 2, Ratings: 2, RatingSeasons: 1,
		Comments: 2, CommentSeasons: 1,
		Groups: 2, GroupMembers: 3, GroupTitles: 2, GroupTitleSeasons: 2,
	}, stats)

	t.Run("preserves rating id and timestamps", func(t *testing.T) {
		var id string
		var createdAt time.Time
		var note float32
		err := pool.QueryRow(ctx,
			"SELECT id, created_at, note FROM ratings WHERE user_id = $1", fixU1).
			Scan(&id, &createdAt, &note)
		require.NoError(t, err)
		assert.Equal(t, fixR1, id)
		assert.True(t, createdAt.Equal(ts(44)), "created_at %v != %v", createdAt, ts(44))
		assert.Equal(t, float32(8.5), note)
	})

	t.Run("preserves soft-deleted group", func(t *testing.T) {
		var deleted bool
		var deletedAt *time.Time
		err := pool.QueryRow(ctx,
			"SELECT deleted, deleted_at FROM groups WHERE id = $1", fixG2).
			Scan(&deleted, &deletedAt)
		require.NoError(t, err)
		assert.True(t, deleted)
		require.NotNil(t, deletedAt)
		assert.True(t, deletedAt.Equal(ts(50)))
	})

	t.Run("store reads the migrated series title", func(t *testing.T) {
		st := postgres.New(pool)
		title, err := st.GetTitleById(ctx, fixT2)
		require.NoError(t, err)
		assert.Equal(t, "Fixture Show", title.PrimaryTitle)
		assert.Len(t, title.Seasons, 2)
		require.NotNil(t, title.AddedAt)
		assert.True(t, title.AddedAt.Equal(ts(20)))
	})

	t.Run("store reads the live group as a member", func(t *testing.T) {
		st := postgres.New(pool)
		group, err := st.GetGroupById(ctx, fixG1, fixU2)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{fixU1, fixU2}, group.Users)
		require.Contains(t, group.Titles, fixT2)
		item := group.Titles[fixT2]
		require.NotNil(t, item.SeasonsWatched)
		assert.Len(t, *item.SeasonsWatched, 2)
	})
}

func TestLoad_RefusesNonEmptyTarget(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)
	dump := loadFixtureViaFiles(t)

	_, err := Load(ctx, pool, dump, false)
	require.NoError(t, err)

	_, err = Load(ctx, pool, dump, false)
	require.ErrorIs(t, err, ErrTargetNotEmpty)

	var users int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users))
	assert.Equal(t, 2, users, "refused run must not change data")
}

func TestLoad_ResetReloads(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pool := newTestPool(t)
	dump := loadFixtureViaFiles(t)

	_, err := Load(ctx, pool, dump, false)
	require.NoError(t, err)

	stats, err := Load(ctx, pool, dump, true)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Users)

	var users, members int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM group_members").Scan(&members))
	assert.Equal(t, 2, users, "reset run must not duplicate rows")
	assert.Equal(t, 3, members)
}
