package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// newTestMovieRating builds a minimal, valid models.UserRating for a movie
// (no SeasonsRatings), matching what services/ratings.addRatingForMovie
// passes to Store.AddRating (no Id/CreatedAt/UpdatedAt: those are generated
// by the store).
func newTestMovieRating(t *testing.T, titleId, userId string, note float32) models.UserRating {
	t.Helper()
	return models.UserRating{
		TitleId: titleId,
		UserId:  userId,
		Note:    note,
	}
}

// newTestSeriesRating builds a models.UserRating with the given season ->
// rating entries, matching what services/ratings.addRatingForTVSeries
// passes to Store.AddRating.
func newTestSeriesRating(t *testing.T, titleId, userId string, seasons map[string]float32) models.UserRating {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	seasonsRatings := make(models.SeasonsRatings, len(seasons))
	var sum float32
	for season, rating := range seasons {
		seasonsRatings[season] = models.SeasonRatingItem{
			Rating:    rating,
			AddedAt:   now,
			UpdatedAt: now,
		}
		sum += rating
	}
	return models.UserRating{
		TitleId:        titleId,
		UserId:         userId,
		Note:           sum / float32(len(seasons)),
		SeasonsRatings: &seasonsRatings,
	}
}

func TestStore_AddRating_GetRatingById_Movie(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-movie-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	rating := newTestMovieRating(t, titleId, userId, 7.5)

	added, err := s.AddRating(ctx, rating)
	require.NoError(t, err)
	require.NotEmpty(t, added.Id, "AddRating must generate an id")
	require.Equal(t, titleId, added.TitleId)
	require.Equal(t, userId, added.UserId)
	require.Equal(t, float32(7.5), added.Note)
	require.Nil(t, added.SeasonsRatings, "a movie rating (no seasons) must round-trip with a nil SeasonsRatings map")
	require.WithinDuration(t, time.Now(), added.CreatedAt, 5*time.Second)
	require.WithinDuration(t, time.Now(), added.UpdatedAt, 5*time.Second)

	got, err := s.GetRatingById(ctx, added.Id, userId)
	require.NoError(t, err)
	require.Equal(t, added, got)
}

func TestStore_AddRating_GetRatingById_Series(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-series-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	rating := newTestSeriesRating(t, titleId, userId, map[string]float32{"1": 8.0, "2": 6.0})

	added, err := s.AddRating(ctx, rating)
	require.NoError(t, err)
	require.NotNil(t, added.SeasonsRatings, "a series rating with seasons must round-trip with a non-nil SeasonsRatings map")
	require.Len(t, *added.SeasonsRatings, 2)
	require.Equal(t, float32(8.0), (*added.SeasonsRatings)["1"].Rating)
	require.Equal(t, float32(6.0), (*added.SeasonsRatings)["2"].Rating)

	got, err := s.GetRatingById(ctx, added.Id, userId)
	require.NoError(t, err)
	require.NotNil(t, got.SeasonsRatings)
	require.Len(t, *got.SeasonsRatings, 2)
	require.Equal(t, (*added.SeasonsRatings)["1"].Rating, (*got.SeasonsRatings)["1"].Rating)
	require.Equal(t, (*added.SeasonsRatings)["2"].Rating, (*got.SeasonsRatings)["2"].Rating)
	require.WithinDuration(t, (*added.SeasonsRatings)["1"].AddedAt, (*got.SeasonsRatings)["1"].AddedAt, time.Second)
}

func TestStore_GetRatingByUserIdAndTitleId(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	added, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, 5.0))
	require.NoError(t, err)

	got, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId)
	require.NoError(t, err)
	require.Equal(t, added, got)

	t.Run("not found", func(t *testing.T) {
		_, err := s.GetRatingByUserIdAndTitleId(ctx, "missing-user", "missing-title")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}

func TestStore_AddRating_Duplicate(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-dup"
	userId := "user-dup"
	rating := newTestMovieRating(t, titleId, userId, 5.0)

	_, err := s.AddRating(ctx, rating)
	require.NoError(t, err)

	_, err = s.AddRating(ctx, rating)
	require.ErrorIs(t, err, store.ErrDuplicatedRecord)
}

func TestStore_GetRatingById_NotFound(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetRatingById(ctx, "missing-rating", "missing-user")
	require.ErrorIs(t, err, store.ErrRecordNotFound)
}

func TestStore_UpdateRating_ReplacesSeasonSet(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	added, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, map[string]float32{
		"1": 5.0, "2": 6.0,
	}))
	require.NoError(t, err)
	require.Len(t, *added.SeasonsRatings, 2)

	now := time.Now().UTC().Truncate(time.Second)
	newSeasons := models.SeasonsRatings{
		"2": {Rating: 7.0, AddedAt: (*added.SeasonsRatings)["2"].AddedAt, UpdatedAt: now},
		"3": {Rating: 9.0, AddedAt: now, UpdatedAt: now},
	}
	update := models.UserRating{
		Id:             added.Id,
		Note:           8.0,
		SeasonsRatings: &newSeasons,
	}

	updated, err := s.UpdateRating(ctx, update, userId)
	require.NoError(t, err)
	require.NotNil(t, updated.SeasonsRatings)
	require.Len(t, *updated.SeasonsRatings, 2)
	require.NotContains(t, *updated.SeasonsRatings, "1", "season 1 must have been dropped by the whole-map replace")
	require.Contains(t, *updated.SeasonsRatings, "2")
	require.Contains(t, *updated.SeasonsRatings, "3")
	require.Equal(t, float32(7.0), (*updated.SeasonsRatings)["2"].Rating)
	require.Equal(t, float32(9.0), (*updated.SeasonsRatings)["3"].Rating)
	require.Equal(t, float32(8.0), updated.Note)

	// Re-read from scratch must reflect exactly the new map, proving the
	// delete+reinsert was actually persisted (not just returned in-memory).
	reread, err := s.GetRatingById(ctx, added.Id, userId)
	require.NoError(t, err)
	require.NotNil(t, reread.SeasonsRatings)
	require.Len(t, *reread.SeasonsRatings, 2)
	require.NotContains(t, *reread.SeasonsRatings, "1")
	require.Equal(t, float32(7.0), (*reread.SeasonsRatings)["2"].Rating)
	require.Equal(t, float32(9.0), (*reread.SeasonsRatings)["3"].Rating)
}

func TestStore_UpdateRating_MovieDropsToNilSeasons(t *testing.T) {
	// Movie ratings never have SeasonsRatings; updating one (note-only)
	// must round-trip with SeasonsRatings still nil.
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	added, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, 5.0))
	require.NoError(t, err)

	updated, err := s.UpdateRating(ctx, models.UserRating{Id: added.Id, Note: 9.0}, userId)
	require.NoError(t, err)
	require.Nil(t, updated.SeasonsRatings)
	require.Equal(t, float32(9.0), updated.Note)
}

func TestStore_UpdateRating_NotFound(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.UpdateRating(ctx, models.UserRating{Id: "missing-rating", Note: 5.0}, "missing-user")
	require.ErrorIs(t, err, store.ErrRecordNotFound)
}

func TestStore_GetRatingsByTitleId(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleId := "tt-" + uuid.NewString()
	otherTitleId := "tt-other-" + uuid.NewString()

	r1, err := s.AddRating(ctx, newTestMovieRating(t, titleId, "user-1", 5.0))
	require.NoError(t, err)
	r2, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, "user-2", map[string]float32{"1": 8.0}))
	require.NoError(t, err)
	_, err = s.AddRating(ctx, newTestMovieRating(t, otherTitleId, "user-3", 3.0))
	require.NoError(t, err)

	got, err := s.GetRatingsByTitleId(ctx, titleId)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byId := map[string]models.UserRating{}
	for _, r := range got {
		byId[r.Id] = r
	}
	require.Contains(t, byId, r1.Id)
	require.Contains(t, byId, r2.Id)
	require.Nil(t, byId[r1.Id].SeasonsRatings)
	require.NotNil(t, byId[r2.Id].SeasonsRatings)
	require.Len(t, *byId[r2.Id].SeasonsRatings, 1)

	empty, err := s.GetRatingsByTitleId(ctx, "no-such-title")
	require.NoError(t, err)
	require.Equal(t, []models.UserRating{}, empty)
}

func TestStore_GetRatingsByTitleIds_BatchGrouping(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	titleA := "tt-a-" + uuid.NewString()
	titleB := "tt-b-" + uuid.NewString()
	titleC := "tt-c-" + uuid.NewString()

	rA, err := s.AddRating(ctx, newTestSeriesRating(t, titleA, "user-1", map[string]float32{"1": 7.0, "2": 8.0}))
	require.NoError(t, err)
	rB, err := s.AddRating(ctx, newTestMovieRating(t, titleB, "user-2", 6.0))
	require.NoError(t, err)
	// titleC has no ratings, and is not included in the query at all.
	_ = titleC

	got, err := s.GetRatingsByTitleIds(ctx, []string{titleA, titleB, titleC})
	require.NoError(t, err)
	require.Len(t, got, 2)

	byId := map[string]models.UserRating{}
	for _, r := range got {
		byId[r.Id] = r
	}
	require.NotNil(t, byId[rA.Id].SeasonsRatings)
	require.Len(t, *byId[rA.Id].SeasonsRatings, 2)
	require.Nil(t, byId[rB.Id].SeasonsRatings)
}

func TestStore_DeleteRating(t *testing.T) {
	t.Run("deletes rating and cascades its seasons", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()
		added, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, map[string]float32{"1": 5.0}))
		require.NoError(t, err)

		count, err := s.DeleteRating(ctx, added.Id, userId)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)

		_, err = s.GetRatingById(ctx, added.Id, userId)
		require.ErrorIs(t, err, store.ErrRecordNotFound)

		var seasonCount int
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM rating_seasons WHERE rating_id = $1`, added.Id).Scan(&seasonCount)
		require.NoError(t, err)
		require.Zero(t, seasonCount, "rating_seasons rows must cascade-delete with their parent rating")
	})

	t.Run("not found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		_, err := s.DeleteRating(ctx, "missing-rating", "missing-user")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}
