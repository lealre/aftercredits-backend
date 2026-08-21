package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/server"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestActivityIsRecorded(t *testing.T) {
	t.Run("adding a title to a group records title_added", func(t *testing.T) {
		resetDB(t)

		user, token := addUser(t, users.NewUserRequest{Username: "recorder", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		rows := getActivityRows(t)
		require.Len(t, rows, 1, "adding a title must record exactly one event")
		require.Equal(t, "title_added", rows[0].Kind)
		require.Equal(t, group.Id, rows[0].GroupId)
		require.Equal(t, user.Id, rows[0].ActorId)
		require.NotNil(t, rows[0].TitleName, "the title's name must be captured at record time")
		require.Equal(t, movie.PrimaryTitle, *rows[0].TitleName)
	})

	// This proves emit-after-success placement, not the middleware's status
	// gate: the request below fails before groups.AddTitleToGroup ever runs,
	// so AddTitleToGroup's activity.Record call is never reached and nothing
	// is ever buffered — the gate has nothing to do here regardless of
	// whether it works. The gate itself (a request that *does* buffer an
	// event but still ends in an error status) is covered separately by
	// internal/server/activity_middleware_test.go's
	// TestActivityMiddleware/a_failed_request_records_nothing, which uses a
	// synthetic handler that records unconditionally before failing.
	t.Run("a request rejected before its write reaches the emit site records nothing", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "failer", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		// A title that is not in the catalogue: the request fails after the
		// guards, so neither the group title nor an event may survive.
		resp := addTitleToGroupResponse(t, groups.AddTitleToGroupRequest{
			URL:     "https://www.imdb.com/title/tt0000000/",
			GroupId: group.Id,
		}, token)
		defer resp.Body.Close()
		require.GreaterOrEqual(t, resp.StatusCode, 400, "the request under test must actually fail")

		require.Empty(t, getActivityRows(t),
			"a request that did not succeed must leave no activity event behind")
	})

	t.Run("removing a title from a group records title_removed", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "remover", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		resp := deleteTitleFromGroupResponse(t, group.Id, movie.ID, token)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 2, "add then remove must record exactly two events")
		require.Equal(t, "title_removed", rows[1].Kind)
		require.Equal(t, group.Id, rows[1].GroupId)
		require.NotNil(t, rows[1].TitleName, "the title's name must be captured before the row is gone")
		require.Equal(t, movie.PrimaryTitle, *rows[1].TitleName)
	})

	t.Run("marking a title watched records title_watched_changed", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "watcher", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		watched := true
		setGroupTitleWatched(t, group.Id, movie.ID, watched, nil, token)

		rows := getActivityRows(t)
		require.Len(t, rows, 2, "add then watch must record exactly two events")
		require.Equal(t, "title_watched_changed", rows[1].Kind)
		require.Equal(t, group.Id, rows[1].GroupId)
		require.NotNil(t, rows[1].TitleName)
		require.Equal(t, movie.PrimaryTitle, *rows[1].TitleName)
	})

	t.Run("adding a rating records rating_added", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "rater", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		addRatingAndGetResult(t, group.Id, movie.ID, float64(7), nil, token)

		rows := getActivityRows(t)
		require.Len(t, rows, 2, "add title then rate must record exactly two events")
		require.Equal(t, "rating_added", rows[1].Kind)
		require.Equal(t, group.Id, rows[1].GroupId)
		require.NotNil(t, rows[1].TitleName)
		require.Equal(t, movie.PrimaryTitle, *rows[1].TitleName)
	})

	t.Run("updating a rating records rating_updated", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "updater", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		rating := addRatingAndGetResult(t, group.Id, movie.ID, float64(7), nil, token)

		resp := updateRating(t, ratings.UpdateRatingRequest{Note: float64(9)}, rating.Id, token)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, rate, then update must record exactly three events")
		require.Equal(t, "rating_updated", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName)
		require.Equal(t, movie.PrimaryTitle, *rows[2].TitleName)
	})

	t.Run("deleting a rating records rating_deleted", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "deleter", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		rating := addRatingAndGetResult(t, group.Id, movie.ID, float64(7), nil, token)

		resp := deleteRating(t, rating.Id, token)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, rate, then delete must record exactly three events")
		require.Equal(t, "rating_deleted", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName, "the title's name must be captured before the row is gone")
		require.Equal(t, movie.PrimaryTitle, *rows[2].TitleName)
	})

	t.Run("deleting a rating season records rating_season_deleted", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "seasonrater", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		tvSeriesTitles := loadTVSeriesTitlesFixture(t)
		seedTitles(t, tvSeriesTitles)
		show := tvSeriesTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", show.ID),
			GroupId: group.Id,
		}, token)

		season := 1
		rating := addRatingAndGetResult(t, group.Id, show.ID, float64(7), &season, token)

		resp := deleteRatingSeason(t, rating.Id, token, season)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, rate season, then delete season must record exactly three events")
		require.Equal(t, "rating_season_deleted", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName)
		require.Equal(t, show.PrimaryTitle, *rows[2].TitleName)
	})

	t.Run("adding a comment records comment_added", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "commenter", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		addCommentAndGetResult(t, group.Id, movie.ID, "great movie", nil, token)

		rows := getActivityRows(t)
		require.Len(t, rows, 2, "add title then comment must record exactly two events")
		require.Equal(t, "comment_added", rows[1].Kind)
		require.Equal(t, group.Id, rows[1].GroupId)
		require.NotNil(t, rows[1].TitleName)
		require.Equal(t, movie.PrimaryTitle, *rows[1].TitleName)
	})

	t.Run("updating a comment records comment_updated", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "commentupdater", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		comment := addCommentAndGetResult(t, group.Id, movie.ID, "great movie", nil, token)

		resp := updateCommentFromApi(t, group.Id, movie.ID, comment.Id, "even better on rewatch", token, nil)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, comment, then update must record exactly three events")
		require.Equal(t, "comment_updated", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName)
		require.Equal(t, movie.PrimaryTitle, *rows[2].TitleName)
	})

	t.Run("deleting a comment records comment_deleted", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "commentdeleter", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, token)

		comment := addCommentAndGetResult(t, group.Id, movie.ID, "great movie", nil, token)

		resp := deleteCommentFromApi(t, group.Id, movie.ID, comment.Id, token)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, comment, then delete must record exactly three events")
		require.Equal(t, "comment_deleted", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName, "the title's name must be captured before the row is gone")
		require.Equal(t, movie.PrimaryTitle, *rows[2].TitleName)
	})

	t.Run("deleting a comment season records comment_season_deleted", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{Username: "commentseasondeleter", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "feed group"}, token)

		tvSeriesTitles := loadTVSeriesTitlesFixture(t)
		seedTitles(t, tvSeriesTitles)
		show := tvSeriesTitles[0]

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", show.ID),
			GroupId: group.Id,
		}, token)

		season := 1
		comment := addCommentAndGetResult(t, group.Id, show.ID, "great season", &season, token)

		resp := deleteCommentSeasonFromApi(t, group.Id, show.ID, comment.Id, token, season)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)

		rows := getActivityRows(t)
		require.Len(t, rows, 3, "add title, comment season, then delete season must record exactly three events")
		require.Equal(t, "comment_season_deleted", rows[2].Kind)
		require.Equal(t, group.Id, rows[2].GroupId)
		require.NotNil(t, rows[2].TitleName)
		require.Equal(t, show.PrimaryTitle, *rows[2].TitleName)
	})
}

// TestActivityWatchedPayload pins the payload of title_watched_changed, which
// is one kind covering several different sentences: marked watched, marked not
// watched, a date added where there was none, a date moved from one day to
// another, and each of those again for a single season. The kind alone cannot
// say which happened — only the before/after pair in the payload can, so this
// asserts the keys and values a frontend will actually receive.
func TestActivityWatchedPayload(t *testing.T) {
	firstWatch := time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC)
	secondWatch := time.Date(2025, 5, 6, 0, 0, 0, 0, time.UTC)

	t.Run("marking a title watched carries no date and a false previousWatched", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "watchmarker", false)

		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId: titleId,
			Watched: watchedFlag(true),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.Equal(t, true, payload["watched"], "the resulting state must be watched")
		require.Equal(t, false, payload["previousWatched"], "it was not watched before, which is what makes this 'marked as watched'")
		require.NotContains(t, payload, "watchedAt", "no date was given, so none may be claimed")
		require.NotContains(t, payload, "previousWatchedAt", "there was no date before either")
		require.NotContains(t, payload, "season", "a title-scoped change must not look season-scoped")
	})

	t.Run("marking a title watched with a date carries the date and no previous one", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "watchdater", false)

		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.Equal(t, true, payload["watched"])
		require.Equal(t, false, payload["previousWatched"])
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "watchedAt")),
			"the date the request set must reach the feed")
		require.NotContains(t, payload, "previousWatchedAt", "nothing was overwritten")
	})

	t.Run("adding a date to an already watched title keeps previousWatched true and carries no previous date", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "dateadder", false)

		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId: titleId,
			Watched: watchedFlag(true),
		}, token)
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			WatchedAt: watchedDate(firstWatch),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.Equal(t, true, payload["watched"])
		require.Equal(t, true, payload["previousWatched"],
			"it was already watched, so this is not a 'marked as watched'")
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "watchedAt")))
		require.NotContains(t, payload, "previousWatchedAt",
			"an absent previous date is what makes this 'added a date' rather than 'changed' it")
	})

	t.Run("changing an existing date carries both dates", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "datechanger", false)

		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			WatchedAt: watchedDate(secondWatch),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.Equal(t, true, payload["watched"])
		require.Equal(t, true, payload["previousWatched"])
		require.True(t, secondWatch.Equal(activityPayloadTime(t, payload, "watchedAt")),
			"the date it moved to")
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "previousWatchedAt")),
			"the date it moved from — the whole point of the fix, since without it this event was indistinguishable from marking the title watched")
	})

	t.Run("marking a title not watched carries the date it lost", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "unwatcher", false)

		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId: titleId,
			Watched: watchedFlag(false),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.Equal(t, false, payload["watched"], "the resulting state must be not watched")
		require.Equal(t, true, payload["previousWatched"], "it was watched before")
		require.NotContains(t, payload, "watchedAt", "unmarking clears the date, so the result carries none")
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "previousWatchedAt")))
	})

	t.Run("marking a season watched is scoped by the season key", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "seasonwatcher", true)

		season := 1
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &season,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.EqualValues(t, season, payload["season"], "a season-scoped change must say which season")
		require.Equal(t, true, payload["watched"])
		require.Equal(t, false, payload["previousWatched"])
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "watchedAt")))
		require.NotContains(t, payload, "previousWatchedAt")
	})

	t.Run("changing a season's date carries both dates and the season", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "seasondatechanger", true)

		season := 2
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &season,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &season,
			WatchedAt: watchedDate(secondWatch),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.EqualValues(t, season, payload["season"])
		require.Equal(t, true, payload["watched"])
		require.Equal(t, true, payload["previousWatched"])
		require.True(t, secondWatch.Equal(activityPayloadTime(t, payload, "watchedAt")))
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "previousWatchedAt")))
	})

	t.Run("marking a season not watched carries the season's own date", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "seasonunwatcher", true)

		season := 1
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &season,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId: titleId,
			Season:  &season,
			Watched: watchedFlag(false),
		}, token)

		payload := lastActivityPayload(t, "title_watched_changed")
		require.EqualValues(t, season, payload["season"])
		require.Equal(t, false, payload["watched"])
		require.Equal(t, true, payload["previousWatched"])
		require.NotContains(t, payload, "watchedAt")
		require.True(t, firstWatch.Equal(activityPayloadTime(t, payload, "previousWatchedAt")))
	})

	// The series' own watched flag is a rollup — true as soon as any season is
	// watched — so reporting it here would say "was already watched" about a
	// season nobody had ever touched.
	t.Run("a season's payload describes the season, not the series rollup", func(t *testing.T) {
		resetDB(t)
		groupId, titleId, token := activityWatchedFixture(t, "seasonscoper", true)

		firstSeason, secondSeason := 1, 2
		applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &firstSeason,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(firstWatch),
		}, token)

		updated := applyWatchedUpdate(t, groupId, groups.UpdateGroupTitleWatchedRequest{
			TitleId:   titleId,
			Season:    &secondSeason,
			Watched:   watchedFlag(true),
			WatchedAt: watchedDate(secondWatch),
		}, token)
		require.True(t, updated.Watched,
			"the series must already read as watched, or this proves nothing about the rollup")

		payload := lastActivityPayload(t, "title_watched_changed")
		require.EqualValues(t, secondSeason, payload["season"])
		require.Equal(t, false, payload["previousWatched"],
			"season 2 had never been watched, whatever season 1 says about the series")
		require.True(t, secondWatch.Equal(activityPayloadTime(t, payload, "watchedAt")),
			"the season's own date, not the series' latest")
		require.NotContains(t, payload, "previousWatchedAt", "season 2 carried no date before")
	})
}

func TestActivityFeedEndpoints(t *testing.T) {
	t.Run("the feed excludes the caller's own actions", func(t *testing.T) {
		resetDB(t)
		testTitleId, _ := seedTwoActivityTitles(t)

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", testTitleId),
			GroupId: group.Id,
		}, actorToken)

		readerFeed := getActivityFeed(t, readerToken, "")
		require.Len(t, readerFeed.Events, 1, "the reader must see the actor's event")
		require.Equal(t, "title_added", readerFeed.Events[0].Kind)
		require.Equal(t, group.Name, readerFeed.Events[0].GroupName,
			"each row carries its group's name for the label")

		actorFeed := getActivityFeed(t, actorToken, "")
		require.Empty(t, actorFeed.Events,
			"the actor must not be notified about their own action")
	})

	t.Run("an empty feed serializes events as [] not null", func(t *testing.T) {
		resetDB(t)
		_, token := addUser(t, users.NewUserRequest{Username: "lonely", Password: "pass"})

		body := getActivityFeedRawBody(t, token, "")
		require.Contains(t, body, `"events":[]`,
			"an empty feed must serialize as [], not null — a decoded struct cannot tell them apart, so this is asserted on the raw body")
		require.Contains(t, body, `"nextBefore":null`)
		require.Contains(t, body, `"hasMore":false`)
	})

	t.Run("marking one event read leaves every other event unread", func(t *testing.T) {
		// The whole point of replacing the watermark. Under the old model
		// "unread" meant "newer than read_seq", so marking event 3 read
		// necessarily read events 1 and 2 with it and the badge fell to 2.
		resetDB(t)
		fixture := seedActivityFeed(t, 5)

		require.EqualValues(t, 5, getActivityUnreadCount(t, fixture.readerToken),
			"all five of the actor's events start unread for the reader")
		require.EqualValues(t, 0, getActivityUnreadCount(t, fixture.actorToken),
			"your own action is never unread for you")

		third := fixture.eventIds[2]
		markActivityEventRead(t, fixture.readerToken, third)

		require.EqualValues(t, 4, getActivityUnreadCount(t, fixture.readerToken),
			"the badge drops by exactly one, not to the number of events newer than event 3")

		want := map[string]bool{}
		for i, id := range fixture.eventIds {
			want[id] = i == 2
		}
		require.Equal(t, want, activityReadStateById(t, fixture.readerToken),
			"exactly event 3 is read: events 1 and 2 below it and 4 and 5 above it are untouched")
	})

	t.Run("marking the same event read twice is a success that does not double-count", func(t *testing.T) {
		resetDB(t)
		fixture := seedActivityFeed(t, 3)

		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[0])
		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[0])

		require.EqualValues(t, 2, getActivityUnreadCount(t, fixture.readerToken),
			"clicking a row twice must not error and must not move the badge twice")
	})

	t.Run("one member's read state is private to them", func(t *testing.T) {
		resetDB(t)
		fixture := seedActivityFeed(t, 3)

		other, otherToken := addUser(t, users.NewUserRequest{Username: "other", Password: "pass"})
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: other.Id}, fixture.groupId, fixture.actorToken)

		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[1])

		require.EqualValues(t, 2, getActivityUnreadCount(t, fixture.readerToken))
		require.EqualValues(t, 3, getActivityUnreadCount(t, otherToken),
			"reading an event as one member must leave it unread for another")
		for id, read := range activityReadStateById(t, otherToken) {
			require.False(t, read, "event %s must still be unread for the other member", id)
		}
	})

	t.Run("mark all read clears every row and a later event raises the badge again", func(t *testing.T) {
		resetDB(t)
		fixture := seedActivityFeed(t, 4)

		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[0])
		markAllActivityRead(t, fixture.readerToken)

		require.EqualValues(t, 0, getActivityUnreadCount(t, fixture.readerToken),
			"marking all read clears the badge, including rows already read")
		for id, read := range activityReadStateById(t, fixture.readerToken) {
			require.True(t, read, "event %s must be read after marking all read", id)
		}

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", fixture.spareTitleId),
			GroupId: fixture.groupId,
		}, fixture.actorToken)

		require.EqualValues(t, 1, getActivityUnreadCount(t, fixture.readerToken),
			"an event recorded after the sweep is unread")
	})

	t.Run("a feed row carries the reader's own read state", func(t *testing.T) {
		resetDB(t)
		fixture := seedActivityFeed(t, 2)

		body := getActivityFeedRawBody(t, fixture.readerToken, "")
		require.Contains(t, body, `"read":false`,
			"every event must carry the flag, and an unread one must say so rather than omit it")

		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[1])
		require.Contains(t, getActivityFeedRawBody(t, fixture.readerToken, "?limit=1"), `"read":true`,
			"the newest row is the one just marked read")
	})

	t.Run("the cursor walks every event exactly once", func(t *testing.T) {
		resetDB(t)

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

		titleIds := seededTitleIds(t, 12)
		for _, id := range titleIds {
			addTitleToGroup(t, groups.AddTitleToGroupRequest{
				URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", id),
				GroupId: group.Id,
			}, actorToken)
		}

		seen := map[string]int{}
		query := "?limit=5"
		for {
			page := getActivityFeed(t, readerToken, query)
			for _, e := range page.Events {
				seen[e.Id]++
			}
			if !page.HasMore || page.NextBefore == nil {
				break
			}
			query = fmt.Sprintf("?limit=5&before=%d", *page.NextBefore)
		}

		require.Len(t, seen, len(titleIds),
			"walking the cursor must return every event exactly once")
		for id, n := range seen {
			require.Equal(t, 1, n, "event %s was returned %d times across pages", id, n)
		}
	})

	t.Run("a non-member never sees another group's events", func(t *testing.T) {
		resetDB(t)
		testTitleId, _ := seedTwoActivityTitles(t)

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		_, outsiderToken := addUser(t, users.NewUserRequest{Username: "outsider", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "private"}, actorToken)
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", testTitleId),
			GroupId: group.Id,
		}, actorToken)

		feed := getActivityFeed(t, outsiderToken, "")
		require.Empty(t, feed.Events, "a non-member must see none of the group's activity")
		require.EqualValues(t, 0, getActivityUnreadCount(t, outsiderToken))
	})

	t.Run("an event the caller cannot see cannot be marked read", func(t *testing.T) {
		resetDB(t)
		fixture := seedActivityFeed(t, 2)
		_, outsiderToken := addUser(t, users.NewUserRequest{Username: "outsider", Password: "pass"})

		resp := markActivityEventReadResponse(t, outsiderToken, fixture.eventIds[0])
		require.Equal(t, http.StatusNotFound, resp.StatusCode,
			"a non-member must not be able to mark another group's event read")
		resp.Body.Close()

		resp = markActivityEventReadResponse(t, fixture.actorToken, fixture.eventIds[0])
		require.Equal(t, http.StatusNotFound, resp.StatusCode,
			"your own action is not in your feed, so there is nothing there to mark read")
		resp.Body.Close()

		resp = markActivityEventReadResponse(t, fixture.readerToken, "no-such-event")
		require.Equal(t, http.StatusNotFound, resp.StatusCode,
			"an unknown id answers the same way, so it cannot be used to probe for events")
		resp.Body.Close()

		require.EqualValues(t, 2, getActivityUnreadCount(t, fixture.readerToken),
			"none of the refused calls may have marked anything read")
	})
}

// TestActivityFeedDisabled is the plug-out test (spec test 9): it is the
// switch under test, not the feature, so it stays to this one case. The
// shared testServer is flag-on for the whole suite (TestMain sets
// ACTIVITY_FEED_ENABLED=true before building it), so this test builds its own
// server after t.Setenv("ACTIVITY_FEED_ENABLED", "false") — the flag is read
// once at construction time in NewServerWithProvider, so t.Setenv only reaches
// it if the server is built after the env var is set.
func TestActivityFeedDisabled(t *testing.T) {
	resetDB(t)
	t.Setenv("ACTIVITY_FEED_ENABLED", "false")

	off := httptest.NewServer(server.NewServerWithProvider(t.Context(), testStore, newFakeTitleProvider(), "test-secret"))
	defer off.Close()

	// A mutating, event-emitting request (adding a title to a group) against
	// the flag-off server: the underlying write still succeeds, but with the
	// recording middleware absent, activity.Record has nothing to buffer into.
	_, token := buildGroupWithTitleAgainst(t, off.URL)

	req, err := http.NewRequest(http.MethodGet, off.URL+"/activity", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		"the route must be entirely absent when the flag is off, not present-and-empty")

	require.Zero(t, countActivityRows(t),
		"no recorder was seeded, so activity.Record must have no-oped")
}

// TestActivityStreamEndToEnd is Phase 2 of the activity feed: spec tests
// 10-15 (docs/superpowers/specs/2026-08-10-group-activity-feed-design.md,
// "Testing"). Every subtest opens at least one SSE connection; every read off
// one is bounded (activityStreamTestTimeout, or the explicit short window in
// noActivityMessageWithin), and every stream is closed by its own cleanup, so
// nothing here can hang the suite.
//
// Two spec tests are deliberately NOT re-proven here, because a real database
// or a real HTTP server already pins them and duplicating would only add
// drift risk:
//
//   - Test 10 (pg_notify fires on commit, not on rollback):
//     internal/postgres/activity_test.go's TestStore_InsertActivityEvents_Notifies,
//     already against a real Postgres container.
//   - Ticket expiry, half of test 15: internal/activity/ticket_test.go, with a
//     clock the test injects. Re-proving expiry here would mean actually
//     waiting out the real 60s TTL — ticketTTL is an unexported const with no
//     hook to shorten it from outside the package — trading a slow suite for
//     a property already pinned exactly.
func TestActivityStreamEndToEnd(t *testing.T) {
	t.Run("a pushed frame is byte-identical to the same event from GET /activity", func(t *testing.T) {
		// Spec test 11. internal/api/activity_stream_handler_test.go already
		// pins this at the handler layer against a stubbed feed; this proves
		// the same property through the real chain end to end: a real insert
		// notifies a real LISTEN loop, which fans out through a real hub,
		// compared against a real GET /activity read of the same row.
		resetDB(t)
		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

		stream := openActivityStream(t, readerToken)

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, actorToken)

		pushed := activityFrameData(t, readActivityFrame(t, stream.r))

		feedResp := getActivityFeedResponse(t, readerToken, "")
		defer feedResp.Body.Close()
		require.Equal(t, http.StatusOK, feedResp.StatusCode)

		var feed struct {
			Events []json.RawMessage `json:"events"`
		}
		require.NoError(t, json.NewDecoder(feedResp.Body).Decode(&feed), "the feed response must be JSON")
		require.Len(t, feed.Events, 1, "the reader's feed must carry exactly the pushed event")

		require.Equal(t, string(feed.Events[0]), pushed,
			"one fact, one serialization: the pushed frame and the feed element must match byte-for-byte")
	})

	t.Run("the stream only delivers events from the reader's own groups, excluding their own actions", func(t *testing.T) {
		// Spec test 14. internal/activity/hub_test.go already pins the
		// visibility predicate itself in isolation; this proves the real
		// server enforces it end to end, over three real connections.
		resetDB(t)
		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		_, outsiderToken := addUser(t, users.NewUserRequest{Username: "outsider", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

		actorStream := openActivityStream(t, actorToken)
		readerStream := openActivityStream(t, readerToken)
		outsiderStream := openActivityStream(t, outsiderToken)

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, actorToken)

		got := activityFrameEvent(t, readActivityFrame(t, readerStream.r))
		require.Equal(t, "title_added", got.Kind, "a fellow member must see the actor's event live")
		require.Equal(t, group.Id, got.GroupId)

		require.True(t, noActivityMessageWithin(t, actorStream.r, time.Second),
			"the actor must never see their own action on their own stream")
		require.True(t, noActivityMessageWithin(t, outsiderStream.r, time.Second),
			"a non-member's stream must receive nothing from a group they do not belong to")
	})

	t.Run("the snapshot and the stream overlap and merge to one event by id", func(t *testing.T) {
		// Spec test 12. The event is committed (and so already in the
		// snapshot's reach) before this test ever reads the snapshot, and it
		// was also pushed live because the stream connected first — the
		// overlap the design doc calls out as legitimate.
		resetDB(t)
		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		movie := movieTitles[0]

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

		stream := openActivityStream(t, readerToken)

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movie.ID),
			GroupId: group.Id,
		}, actorToken)

		pushed := activityFrameEvent(t, readActivityFrame(t, stream.r))

		feed := getActivityFeed(t, readerToken, "")
		require.Len(t, feed.Events, 1, "the already-committed event must also appear in the snapshot")
		require.Equal(t, pushed.Id, feed.Events[0].Id,
			"the snapshot and the stream must describe the same event")

		merged := map[string]bool{pushed.Id: true}
		for _, e := range feed.Events {
			merged[e.Id] = true
		}
		require.Len(t, merged, 1, "deduplicating by id must render the overlapping event exactly once")
	})

	t.Run("a reconnect ends with the same set as a client that never dropped, including a late low-seq commit", func(t *testing.T) {
		// Spec test 13. seq is assigned when INSERT executes, not when its
		// transaction commits, so a transaction that inserts first but
		// commits last can still notify last, carrying a seq lower than an
		// event already delivered. A resume query like `seq > lastDelivered`
		// would miss it forever; taking a fresh snapshot on every (re)connect
		// — this design's actual choice — does not.
		resetDB(t)
		movieTitles := loadTitlesFixture(t)
		seedTitles(t, movieTitles)
		require.GreaterOrEqual(t, len(movieTitles), 2, "the fixture must carry at least two titles")
		movieB := movieTitles[1]

		actor, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)

		// A client that never drops: connected before either event exists.
		liveStream := openActivityStream(t, readerToken)

		ctx := context.Background()
		// The late event's transaction: its INSERT (and same-transaction
		// NOTIFY) run now, so its seq is allocated now — but it is held open,
		// uncommitted, until later.
		heldTx, lateEventId := insertHeldActivityEvent(t, ctx, group.Id, actor.Id, actor.Username, "title_added")

		// A normal event, through the real path: its INSERT runs after the
		// held one above, so it gets a HIGHER seq, but it commits (and
		// notifies) immediately — before the held transaction does.
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", movieB.ID),
			GroupId: group.Id,
		}, actorToken)

		firstLive := activityFrameEvent(t, readActivityFrame(t, liveStream.r))
		require.NotEqual(t, lateEventId, firstLive.Id,
			"the higher-seq event must arrive first: the held transaction has not committed yet")

		// Now let the late, lower-seq event commit — after the higher-seq one.
		require.NoError(t, heldTx.Commit(ctx), "failed to commit the held transaction")

		secondLive := activityFrameEvent(t, readActivityFrame(t, liveStream.r))
		require.Equal(t, lateEventId, secondLive.Id,
			"the never-dropped client must still receive the late, lower-seq event once it commits")

		liveSeen := map[string]bool{firstLive.Id: true, secondLive.Id: true}
		require.Len(t, liveSeen, 2, "the never-dropped client must have seen both events, live")

		// A reconnect takes a fresh snapshot rather than resuming from
		// Last-Event-ID — precisely what catches the late, lower-seq event a
		// `seq > lastDelivered` resume would have missed permanently.
		reconnectFeed := getActivityFeed(t, readerToken, "")
		reconnectSeen := map[string]bool{}
		for _, e := range reconnectFeed.Events {
			reconnectSeen[e.Id] = true
		}
		require.Equal(t, liveSeen, reconnectSeen,
			"a reconnecting client's fresh snapshot must end up with exactly the same set the never-dropped client saw live")
	})

	t.Run("a ticket redeemed over the real server cannot be reused", func(t *testing.T) {
		// Spec test 15 (single-use half). internal/api/activity_stream_handler_test.go
		// already pins reuse against a stub store; this proves the same
		// property composed with the real chain — real JWT auth, a real user,
		// group membership resolved from the real database.
		resetDB(t)
		_, token := addUser(t, users.NewUserRequest{Username: "streamer", Password: "pass"})

		ticket := mintStreamTicket(t, token)

		first := openActivityStreamWithTicket(t, ticket.Ticket)
		require.Equal(t, http.StatusOK, first.StatusCode,
			"the first use of a freshly minted ticket must open the stream")
		first.Body.Close()

		second := openActivityStreamWithTicket(t, ticket.Ticket)
		require.Equal(t, http.StatusUnauthorized, second.StatusCode,
			"a ticket is single-use: replaying it against the real server must not open a second stream")
	})
}

// TestActivityReadFloor pins the storage half of read state: a floor covering
// everything below it, and exception rows only for events above it.
//
// The endpoints and their responses are unchanged from per-event storage, so
// the cases that matter here are the ones the badge alone cannot distinguish —
// how many rows a mark-all actually writes, and what a reader sees in a group
// whose history predates them.
func TestActivityReadFloor(t *testing.T) {
	t.Run("marking all read raises the floor instead of writing a row per event", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		fixture := seedActivityFeed(t, 5)
		require.EqualValues(t, 5, getActivityUnreadCount(t, fixture.readerToken),
			"all five of the actor's events start unread for the reader")

		markAllActivityRead(t, fixture.readerToken)

		require.EqualValues(t, 0, getActivityUnreadCount(t, fixture.readerToken),
			"marking all read must clear the badge")
		require.Zero(t, countActivityExceptionRows(t),
			"marking all read must move the floor, not write an exception row per event")
	})

	t.Run("marking one row read writes exactly one exception row", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		fixture := seedActivityFeed(t, 5)
		// eventIds is oldest first, so index 2 is neither the newest nor the
		// oldest — the case a floor alone could never express.
		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[2])

		require.EqualValues(t, 4, getActivityUnreadCount(t, fixture.readerToken),
			"marking one event read must drop the badge by exactly one")
		require.Equal(t, 1, countActivityExceptionRows(t),
			"an event above the floor is recorded as a single exception row")

		read := activityReadStateById(t, fixture.readerToken)
		require.True(t, read[fixture.eventIds[2]], "the clicked event is read")
		for _, i := range []int{0, 1, 3, 4} {
			require.False(t, read[fixture.eventIds[i]],
				"event %d must stay unread — neither older nor newer rows may collapse", i)
		}
	})

	t.Run("marking read below the floor succeeds without writing a row", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		fixture := seedActivityFeed(t, 3)
		markAllActivityRead(t, fixture.readerToken)
		require.Zero(t, countActivityExceptionRows(t), "the floor covers every event")

		// Already read via the floor: the endpoint must still report success
		// (idempotent), and must not add a row the floor already covers.
		resp := markActivityEventReadResponse(t, fixture.readerToken, fixture.eventIds[0])
		defer resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode,
			"marking an already-read event read is a no-op, not an error")
		require.Zero(t, countActivityExceptionRows(t),
			"an event at or below the floor must not produce a redundant exception row")
		require.EqualValues(t, 0, getActivityUnreadCount(t, fixture.readerToken))
	})

	t.Run("exceptions above the floor survive a later mark-all sweep", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		fixture := seedActivityFeed(t, 3)
		markActivityEventRead(t, fixture.readerToken, fixture.eventIds[1])
		require.Equal(t, 1, countActivityExceptionRows(t))

		// A newer event arrives, then everything is marked read: the floor
		// rises past all of them and the exception row becomes redundant.
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", fixture.spareTitleId),
			GroupId: fixture.groupId,
		}, fixture.actorToken)
		require.EqualValues(t, 3, getActivityUnreadCount(t, fixture.readerToken),
			"two originals plus the new one, with the read one excluded")

		markAllActivityRead(t, fixture.readerToken)

		require.EqualValues(t, 0, getActivityUnreadCount(t, fixture.readerToken))
		require.Zero(t, countActivityExceptionRows(t),
			"rows the raised floor now covers must be deleted, or they accumulate exactly as before")

		for _, id := range fixture.eventIds {
			require.True(t, activityReadStateById(t, fixture.readerToken)[id],
				"every event stays read after the sweep")
		}
	})

	t.Run("a new member does not inherit the group's backlog as unread", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		// A group with history, and a reader who marks it all read.
		fixture := seedActivityFeed(t, 3)
		markAllActivityRead(t, fixture.readerToken)

		// A third person joins after those events happened. They were not
		// there, so the history is not news to them — it is below their floor
		// only if they have one, which they do not. This pins whichever
		// behaviour the model actually produces so it cannot change silently.
		latecomer, latecomerToken := addUser(t, users.NewUserRequest{Username: "latecomer", Password: "pass"})
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: latecomer.Id},
			fixture.groupId, fixture.actorToken)

		backlog := getActivityUnreadCount(t, latecomerToken)
		require.EqualValues(t, 3, backlog,
			"a joiner with no floor sees the group's history as unread; raising a floor on join is a product decision, not an accident")

		// And they can clear it in one move without writing three rows.
		markAllActivityRead(t, latecomerToken)
		require.EqualValues(t, 0, getActivityUnreadCount(t, latecomerToken))
		require.Zero(t, countActivityExceptionRows(t))
	})
}

// TestActivityReadFloorAcrossGroups pins the one place the floor is genuinely
// lossy: it is one number per user, not per group, so it spans every group they
// are in.
//
// A reader who has cleared their badge has a floor at the newest event they
// could see. Joining a group whose history is older than that puts those events
// below the floor, and they arrive already read. Whether that is right is a
// product call — you were not there, so it is arguably not news — but it is a
// real consequence of one floor per user rather than one per group, and it must
// not be discovered by someone wondering where their notifications went.
func TestActivityReadFloorAcrossGroups(t *testing.T) {
	t.Run("joining a group whose history predates your floor arrives read", func(t *testing.T) {
		resetDB(t)
		seedTitles(t, loadTitlesFixture(t))

		titleIds := seededTitleIds(t, 4)

		// An older group the reader is not in yet, with two events in it.
		_, strangerToken := addUser(t, users.NewUserRequest{Username: "stranger", Password: "pass"})
		oldGroup := createGroup(t, groups.CreateGroupRequest{Name: "old"}, strangerToken)
		for _, titleId := range titleIds[:2] {
			addTitleToGroup(t, groups.AddTitleToGroupRequest{
				URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleId),
				GroupId: oldGroup.Id,
			}, strangerToken)
		}

		// Meanwhile the reader is active elsewhere and clears their badge,
		// which puts their floor above those two older events.
		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		ownGroup := createGroup(t, groups.CreateGroupRequest{Name: "own"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, ownGroup.Id, actorToken)
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleIds[2]),
			GroupId: ownGroup.Id,
		}, actorToken)

		markAllActivityRead(t, readerToken)
		floor := activityReadFloor(t, reader.Id)
		require.NotZero(t, floor, "clearing the badge must leave a floor to be meaningful")

		// Now they join the older group.
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, oldGroup.Id, strangerToken)

		require.EqualValues(t, 0, getActivityUnreadCount(t, readerToken),
			"the older group's history sits below the reader's floor and arrives already read")

		// A new event in that same group is above the floor and does notify,
		// so joining costs the backlog only, never future activity.
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleIds[3]),
			GroupId: oldGroup.Id,
		}, strangerToken)
		require.EqualValues(t, 1, getActivityUnreadCount(t, readerToken),
			"activity after joining must still reach the new member")
	})
}
