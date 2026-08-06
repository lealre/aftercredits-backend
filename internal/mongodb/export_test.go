package mongodb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportedDbToModelWrappers(t *testing.T) {
	created := time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)
	watchedAt := created.Add(2 * time.Minute)
	avatar := "https://example.com/a.png"

	t.Run("UserDbToModel", func(t *testing.T) {
		got := UserDbToModel(UserDb{
			Id: "u1", Name: "Alice", Email: "alice@example.com", Username: "alice",
			PasswordHash: "hash", AvatarURL: &avatar, Groups: []string{"g1"},
			Role: "admin", IsActive: true, CreatedAt: created, UpdatedAt: updated,
		})
		assert.Equal(t, "u1", got.Id)
		assert.Equal(t, "alice", got.Username)
		require.NotNil(t, got.AvatarURL)
		assert.Equal(t, avatar, *got.AvatarURL)
		assert.Equal(t, []string{"g1"}, got.Groups)
		assert.True(t, got.CreatedAt.Equal(created))
	})

	t.Run("GroupDbToModel maps titles and seasons", func(t *testing.T) {
		seasons := SeasonWatchedDb{"1": {Watched: true, WatchedAt: &watchedAt, AddedAt: created, UpdatedAt: updated}}
		got := GroupDbToModel(GroupDb{
			Id: "g1", Name: "night", OwnerId: "u1", Users: UsersIds{"u1", "u2"},
			Titles: GroupTitleDb{"tt1": {TitleId: "tt1", TitleType: "tvSeries",
				SeasonsWatched: &seasons, Watched: true, AddedAt: created, UpdatedAt: updated, WatchedAt: &watchedAt}},
			CreatedAt: created, UpdatedAt: updated,
		})
		assert.Equal(t, []string{"u1", "u2"}, got.Users)
		require.Contains(t, got.Titles, "tt1")
		item := got.Titles["tt1"]
		require.NotNil(t, item.SeasonsWatched)
		assert.True(t, (*item.SeasonsWatched)["1"].Watched)
	})

	t.Run("UserRatingDbToModel keeps nil seasons nil", func(t *testing.T) {
		got := UserRatingDbToModel(RatingDb{Id: "r1", TitleId: "tt1", UserId: "u1",
			Note: 8.5, CreatedAt: created, UpdatedAt: updated})
		assert.Nil(t, got.SeasonsRatings)
		assert.Equal(t, float32(8.5), got.Note)
	})

	t.Run("CommentDbToModel keeps comment pointer", func(t *testing.T) {
		text := "loved it"
		got := CommentDbToModel(CommentDb{Id: "c1", TitleId: "tt1", UserId: "u1",
			Comment: &text, CreatedAt: created, UpdatedAt: updated})
		require.NotNil(t, got.Comment)
		assert.Equal(t, text, *got.Comment)
	})

	t.Run("TitleDbToModel returns non-nil slices", func(t *testing.T) {
		got := TitleDbToModel(TitleDb{ID: "tt1", Type: "movie", PrimaryTitle: "Fixture"})
		assert.NotNil(t, got.Directors)
		assert.NotNil(t, got.Seasons)
	})
}
