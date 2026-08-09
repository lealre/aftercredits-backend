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

// addTestGroup creates a fresh owner user plus a group owned by them and
// returns the group's id. Ratings and comments carry a NOT NULL group_id with
// a foreign key to groups(id), so every rating/comment fixture in this package
// needs a real group to belong to. Shared with comments_test.go.
func addTestGroup(t *testing.T, s *Store) string {
	t.Helper()
	owner := addTestUser(t, s)
	group, err := s.CreateGroup(context.Background(), newTestGroup(t, "group-"+uuid.NewString(), owner))
	require.NoError(t, err, "failed to create the group the rating/comment fixtures hang off")
	return group.Id
}

// newTestMovieRating builds a minimal, valid models.UserRating for a movie
// (no SeasonsRatings), matching what services/ratings.addRatingForMovie
// passes to Store.AddRating (no Id/CreatedAt/UpdatedAt: those are generated
// by the store). groupId is part of the rating's identity and is never empty.
func newTestMovieRating(t *testing.T, titleId, userId, groupId string, note float32) models.UserRating {
	t.Helper()
	return models.UserRating{
		TitleId: titleId,
		UserId:  userId,
		GroupId: groupId,
		Note:    note,
	}
}

// newTestSeriesRating builds a models.UserRating with the given season ->
// rating entries, matching what services/ratings.addRatingForTVSeries
// passes to Store.AddRating.
func newTestSeriesRating(t *testing.T, titleId, userId, groupId string, seasons map[string]float32) models.UserRating {
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
		GroupId:        groupId,
		Note:           sum / float32(len(seasons)),
		SeasonsRatings: &seasonsRatings,
	}
}

func TestStore_AddRating_GetRatingById_Movie(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupId := addTestGroup(t, s)
	titleId := "tt-movie-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	rating := newTestMovieRating(t, titleId, userId, groupId, 7.5)

	added, err := s.AddRating(ctx, rating)
	require.NoError(t, err)
	require.NotEmpty(t, added.Id, "AddRating must generate an id")
	require.Equal(t, titleId, added.TitleId)
	require.Equal(t, userId, added.UserId)
	require.Equal(t, groupId, added.GroupId, "the rating must round-trip with the group it was written in")
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

	groupId := addTestGroup(t, s)
	titleId := "tt-series-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	rating := newTestSeriesRating(t, titleId, userId, groupId, map[string]float32{"1": 8.0, "2": 6.0})

	added, err := s.AddRating(ctx, rating)
	require.NoError(t, err)
	require.Equal(t, groupId, added.GroupId, "the rating must round-trip with the group it was written in")
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
	t.Run("round trip", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupId := addTestGroup(t, s)
		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()
		added, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupId, 5.0))
		require.NoError(t, err)

		got, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupId)
		require.NoError(t, err)
		require.Equal(t, added, got)
	})

	t.Run("resolves the requested group's row when the user rated the title twice", func(t *testing.T) {
		// The underlying query is a sqlc :one, which compiles to QueryRow and
		// silently discards every row after the first. Without the group_id
		// predicate this lookup returns an arbitrary one of the two rows.
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupA := addTestGroup(t, s)
		groupB := addTestGroup(t, s)
		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()

		inA, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 4.0))
		require.NoError(t, err)
		inB, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupB, 9.0))
		require.NoError(t, err)

		gotA, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupA)
		require.NoError(t, err)
		require.Equal(t, inA.Id, gotA.Id, "the (user, title) lookup must resolve group A's row, not group B's")
		require.Equal(t, groupA, gotA.GroupId)
		require.Equal(t, float32(4.0), gotA.Note)

		gotB, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupB)
		require.NoError(t, err)
		require.Equal(t, inB.Id, gotB.Id, "the (user, title) lookup must resolve group B's row, not group A's")
		require.Equal(t, groupB, gotB.GroupId)
		require.Equal(t, float32(9.0), gotB.Note)
	})

	t.Run("a rating in another group is not visible to this group", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupA := addTestGroup(t, s)
		groupB := addTestGroup(t, s)
		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()

		_, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 5.0))
		require.NoError(t, err)

		_, err = s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupB)
		require.ErrorIs(t, err, store.ErrRecordNotFound, "group B must not see the rating written in group A")
	})

	t.Run("not found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		_, err := s.GetRatingByUserIdAndTitleId(ctx, "missing-user", "missing-title", "missing-group")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}

func TestStore_AddRating_SameTitleInTwoGroups(t *testing.T) {
	// A rating is a (user, title, group) fact: the same user may rate the same
	// title differently in two groups, and both rows stand on their own.
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupA := addTestGroup(t, s)
	groupB := addTestGroup(t, s)
	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()

	inA, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 3.0))
	require.NoError(t, err)
	inB, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupB, 10.0))
	require.NoError(t, err, "rating a title already rated in another group must be allowed")

	require.NotEqual(t, inA.Id, inB.Id, "the two ratings must be distinct rows")
	require.Equal(t, groupA, inA.GroupId)
	require.Equal(t, groupB, inB.GroupId)

	// Each group's read path sees exactly its own rating, with its own note.
	ratingsA, err := s.GetRatingsByTitleId(ctx, titleId, groupA)
	require.NoError(t, err)
	require.Len(t, ratingsA, 1, "group A must see only its own rating of the title")
	require.Equal(t, inA.Id, ratingsA[0].Id)
	require.Equal(t, float32(3.0), ratingsA[0].Note)

	ratingsB, err := s.GetRatingsByTitleId(ctx, titleId, groupB)
	require.NoError(t, err)
	require.Len(t, ratingsB, 1, "group B must see only its own rating of the title")
	require.Equal(t, inB.Id, ratingsB[0].Id)
	require.Equal(t, float32(10.0), ratingsB[0].Note)

	// Updating one group's rating leaves the other untouched.
	_, err = s.UpdateRating(ctx, models.UserRating{Id: inB.Id, Note: 6.5}, userId)
	require.NoError(t, err)

	rereadA, err := s.GetRatingById(ctx, inA.Id, userId)
	require.NoError(t, err)
	require.Equal(t, float32(3.0), rereadA.Note, "updating group B's rating must not touch group A's")
	rereadB, err := s.GetRatingById(ctx, inB.Id, userId)
	require.NoError(t, err)
	require.Equal(t, float32(6.5), rereadB.Note)
}

func TestStore_AddRating_Duplicate(t *testing.T) {
	t.Run("same user, title and group is a duplicate", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupId := addTestGroup(t, s)
		titleId := "tt-dup-" + uuid.NewString()
		userId := "user-dup-" + uuid.NewString()

		_, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupId, 5.0))
		require.NoError(t, err)

		_, err = s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupId, 7.0))
		require.ErrorIs(t, err, store.ErrDuplicatedRecord, "a second rating of the same title in the same group must be refused")
	})

	t.Run("same user and title in a different group is not a duplicate", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupA := addTestGroup(t, s)
		groupB := addTestGroup(t, s)
		titleId := "tt-dup-" + uuid.NewString()
		userId := "user-dup-" + uuid.NewString()

		_, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 5.0))
		require.NoError(t, err)

		second, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupB, 7.0))
		require.NoError(t, err, "the same (user, title) in another group is a different fact and must be accepted")
		require.Equal(t, groupB, second.GroupId)
	})
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

	groupId := addTestGroup(t, s)
	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	added, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, groupId, map[string]float32{
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
	require.Equal(t, groupId, updated.GroupId, "an update must not move the rating out of its group")
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

func TestStore_SeriesRating_PerSeasonPathsResolveTheRightGroup(t *testing.T) {
	// The TV-series add/update flow looks its row up by (user, title, group)
	// and then replaces that row's whole season map. With two groups holding a
	// rating of the same series, each side must move independently.
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupA := addTestGroup(t, s)
	groupB := addTestGroup(t, s)
	titleId := "tt-series-" + uuid.NewString()
	userId := "user-" + uuid.NewString()

	inA, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, groupA, map[string]float32{"1": 5.0}))
	require.NoError(t, err)
	inB, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, groupB, map[string]float32{"1": 9.0, "2": 8.0}))
	require.NoError(t, err)

	// The add path's "does this user already have a rating here?" lookup must
	// return the group's own row, with the group's own seasons.
	lookupA, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupA)
	require.NoError(t, err)
	require.Equal(t, inA.Id, lookupA.Id)
	require.NotNil(t, lookupA.SeasonsRatings)
	require.Len(t, *lookupA.SeasonsRatings, 1, "group A's row must carry only group A's seasons")
	require.Equal(t, float32(5.0), (*lookupA.SeasonsRatings)["1"].Rating)

	lookupB, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupB)
	require.NoError(t, err)
	require.Equal(t, inB.Id, lookupB.Id)
	require.Len(t, *lookupB.SeasonsRatings, 2, "group B's row must carry only group B's seasons")

	// Add a season to group B's rating, the way the series update path does:
	// take the row it just resolved, extend its map, write it back.
	now := time.Now().UTC().Truncate(time.Second)
	seasonsB := *lookupB.SeasonsRatings
	seasonsB["3"] = models.SeasonRatingItem{Rating: 6.0, AddedAt: now, UpdatedAt: now}
	_, err = s.UpdateRating(ctx, models.UserRating{Id: lookupB.Id, Note: 7.6, SeasonsRatings: &seasonsB}, userId)
	require.NoError(t, err)

	rereadB, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupB)
	require.NoError(t, err)
	require.Len(t, *rereadB.SeasonsRatings, 3)
	require.Equal(t, float32(6.0), (*rereadB.SeasonsRatings)["3"].Rating)

	rereadA, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupA)
	require.NoError(t, err)
	require.Equal(t, inA.Id, rereadA.Id)
	require.Len(t, *rereadA.SeasonsRatings, 1, "group A's seasons must be untouched by an update in group B")
	require.Equal(t, float32(5.0), (*rereadA.SeasonsRatings)["1"].Rating)
}

func TestStore_UpdateRating_MovieDropsToNilSeasons(t *testing.T) {
	// Movie ratings never have SeasonsRatings; updating one (note-only)
	// must round-trip with SeasonsRatings still nil.
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupId := addTestGroup(t, s)
	titleId := "tt-" + uuid.NewString()
	userId := "user-" + uuid.NewString()
	added, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupId, 5.0))
	require.NoError(t, err)

	updated, err := s.UpdateRating(ctx, models.UserRating{Id: added.Id, Note: 9.0}, userId)
	require.NoError(t, err)
	require.Nil(t, updated.SeasonsRatings)
	require.Equal(t, float32(9.0), updated.Note)
	require.Equal(t, groupId, updated.GroupId)
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

	groupId := addTestGroup(t, s)
	otherGroupId := addTestGroup(t, s)
	titleId := "tt-" + uuid.NewString()
	otherTitleId := "tt-other-" + uuid.NewString()

	r1, err := s.AddRating(ctx, newTestMovieRating(t, titleId, "user-1", groupId, 5.0))
	require.NoError(t, err)
	r2, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, "user-2", groupId, map[string]float32{"1": 8.0}))
	require.NoError(t, err)
	_, err = s.AddRating(ctx, newTestMovieRating(t, otherTitleId, "user-3", groupId, 3.0))
	require.NoError(t, err)
	// Same title, another group: must never appear in this group's read.
	_, err = s.AddRating(ctx, newTestMovieRating(t, titleId, "user-4", otherGroupId, 1.0))
	require.NoError(t, err)

	got, err := s.GetRatingsByTitleId(ctx, titleId, groupId)
	require.NoError(t, err)
	require.Len(t, got, 2, "only the requested group's ratings of the title may be returned")

	byId := map[string]models.UserRating{}
	for _, r := range got {
		byId[r.Id] = r
		require.Equal(t, groupId, r.GroupId, "every returned rating must belong to the requested group")
	}
	require.Contains(t, byId, r1.Id)
	require.Contains(t, byId, r2.Id)
	require.Nil(t, byId[r1.Id].SeasonsRatings)
	require.NotNil(t, byId[r2.Id].SeasonsRatings)
	require.Len(t, *byId[r2.Id].SeasonsRatings, 1)

	empty, err := s.GetRatingsByTitleId(ctx, "no-such-title", groupId)
	require.NoError(t, err)
	require.Equal(t, []models.UserRating{}, empty)
}

func TestStore_GetRatingsByTitleIds_BatchGrouping(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupId := addTestGroup(t, s)
	titleA := "tt-a-" + uuid.NewString()
	titleB := "tt-b-" + uuid.NewString()
	titleC := "tt-c-" + uuid.NewString()

	rA, err := s.AddRating(ctx, newTestSeriesRating(t, titleA, "user-1", groupId, map[string]float32{"1": 7.0, "2": 8.0}))
	require.NoError(t, err)
	rB, err := s.AddRating(ctx, newTestMovieRating(t, titleB, "user-2", groupId, 6.0))
	require.NoError(t, err)
	// titleC has no ratings, and is not included in the query at all.
	_ = titleC

	got, err := s.GetRatingsByTitleIds(ctx, []string{titleA, titleB, titleC}, groupId)
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

func TestStore_GetRatingsByTitleIds_OnlyRequestedGroup(t *testing.T) {
	// Regression: this batch read feeds a group's title detail. Without the
	// group_id predicate it shipped every rating on those titles by every user
	// in the system, from every other group.
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	groupA := addTestGroup(t, s)
	groupB := addTestGroup(t, s)
	titleOne := "tt-one-" + uuid.NewString()
	titleTwo := "tt-two-" + uuid.NewString()

	mine1, err := s.AddRating(ctx, newTestMovieRating(t, titleOne, "user-a1", groupA, 7.0))
	require.NoError(t, err)
	mine2, err := s.AddRating(ctx, newTestSeriesRating(t, titleTwo, "user-a2", groupA, map[string]float32{"1": 9.0}))
	require.NoError(t, err)

	// Same titles, same users, other group: all of these must stay invisible.
	theirs1, err := s.AddRating(ctx, newTestMovieRating(t, titleOne, "user-a1", groupB, 1.0))
	require.NoError(t, err)
	theirs2, err := s.AddRating(ctx, newTestMovieRating(t, titleOne, "user-b1", groupB, 2.0))
	require.NoError(t, err)
	theirs3, err := s.AddRating(ctx, newTestMovieRating(t, titleTwo, "user-b2", groupB, 3.0))
	require.NoError(t, err)

	got, err := s.GetRatingsByTitleIds(ctx, []string{titleOne, titleTwo}, groupA)
	require.NoError(t, err)
	require.Len(t, got, 2, "group A must see exactly its own two ratings")

	ids := make([]string, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.Id)
		require.Equal(t, groupA, r.GroupId, "a rating from another group leaked into group A's title detail")
	}
	require.ElementsMatch(t, []string{mine1.Id, mine2.Id}, ids)
	require.NotContains(t, ids, theirs1.Id)
	require.NotContains(t, ids, theirs2.Id)
	require.NotContains(t, ids, theirs3.Id)

	// The mirror image: group B sees only its own three.
	gotB, err := s.GetRatingsByTitleIds(ctx, []string{titleOne, titleTwo}, groupB)
	require.NoError(t, err)
	require.Len(t, gotB, 3, "group B must see exactly its own three ratings")
	for _, r := range gotB {
		require.Equal(t, groupB, r.GroupId)
	}
}

func TestStore_DeleteRating(t *testing.T) {
	t.Run("deletes rating and cascades its seasons", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupId := addTestGroup(t, s)
		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()
		added, err := s.AddRating(ctx, newTestSeriesRating(t, titleId, userId, groupId, map[string]float32{"1": 5.0}))
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

	t.Run("deletes only the targeted group's rating", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupA := addTestGroup(t, s)
		groupB := addTestGroup(t, s)
		titleId := "tt-" + uuid.NewString()
		userId := "user-" + uuid.NewString()

		inA, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 5.0))
		require.NoError(t, err)
		inB, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupB, 8.0))
		require.NoError(t, err)

		count, err := s.DeleteRating(ctx, inA.Id, userId)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)

		_, err = s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupA)
		require.ErrorIs(t, err, store.ErrRecordNotFound)

		survivor, err := s.GetRatingByUserIdAndTitleId(ctx, userId, titleId, groupB)
		require.NoError(t, err, "deleting group A's rating must leave group B's alone")
		require.Equal(t, inB.Id, survivor.Id)
	})

	t.Run("not found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		_, err := s.DeleteRating(ctx, "missing-rating", "missing-user")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}

// TestStore_Ratings_CascadeOnGroupDelete locks in the ON DELETE CASCADE on
// ratings.group_id. A rating is a fact about (user, title, group), so it has no
// meaning once its group is gone, and cascading matches group_members and
// group_titles in 001_init.sql. Nothing in the application hard-deletes a group
// today (SoftDeleteGroup only sets deleted = true), so this asserts the schema
// contract a future purge routine would rely on.
func TestStore_Ratings_CascadeOnGroupDelete(t *testing.T) {
	t.Run("hard-deleting a group removes its ratings", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		groupA := addTestGroup(t, s)
		groupB := addTestGroup(t, s)
		titleId := "tt-cascade-" + uuid.NewString()
		userId := "user-cascade-" + uuid.NewString()

		_, err := s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupA, 8.0))
		require.NoError(t, err, "failed to seed the rating in the group being deleted")
		_, err = s.AddRating(ctx, newTestMovieRating(t, titleId, userId, groupB, 3.0))
		require.NoError(t, err, "failed to seed the rating in the surviving group")

		_, err = newTestPool(t).Exec(ctx, `DELETE FROM groups WHERE id = $1`, groupA)
		require.NoError(t, err, "hard-deleting a group must succeed, not be blocked by the ratings foreign key")

		gone, err := s.GetRatingsByTitleId(ctx, titleId, groupA)
		require.NoError(t, err)
		require.Empty(t, gone, "the deleted group's ratings must be gone")

		kept, err := s.GetRatingsByTitleId(ctx, titleId, groupB)
		require.NoError(t, err)
		require.Len(t, kept, 1, "the other group's rating must survive the cascade")
	})
}
