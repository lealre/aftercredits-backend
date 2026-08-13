package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

		addRatingAndGetResult(t, group.Id, movie.ID, float32(7), nil, token)

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

		rating := addRatingAndGetResult(t, group.Id, movie.ID, float32(7), nil, token)

		resp := updateRating(t, ratings.UpdateRatingRequest{Note: float32(9)}, rating.Id, token)
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

		rating := addRatingAndGetResult(t, group.Id, movie.ID, float32(7), nil, token)

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
		rating := addRatingAndGetResult(t, group.Id, show.ID, float32(7), &season, token)

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

	t.Run("unread count drops to zero after marking read and rises again", func(t *testing.T) {
		resetDB(t)
		testTitleId, secondTestTitleId := seedTwoActivityTitles(t)

		_, actorToken := addUser(t, users.NewUserRequest{Username: "actor", Password: "pass"})
		reader, readerToken := addUser(t, users.NewUserRequest{Username: "reader", Password: "pass"})
		group := createGroup(t, groups.CreateGroupRequest{Name: "shared"}, actorToken)
		addUserToGroup(t, groups.AddUserToGroupRequest{UserId: reader.Id}, group.Id, actorToken)
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", testTitleId),
			GroupId: group.Id,
		}, actorToken)

		require.EqualValues(t, 1, getActivityUnreadCount(t, readerToken),
			"the actor's event is unread for the reader")
		require.EqualValues(t, 0, getActivityUnreadCount(t, actorToken),
			"your own action is never unread for you")

		feed := getActivityFeed(t, readerToken, "")
		markActivityRead(t, readerToken, feed.Events[0].Seq)
		require.EqualValues(t, 0, getActivityUnreadCount(t, readerToken),
			"marking the newest seq read clears the badge")

		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", secondTestTitleId),
			GroupId: group.Id,
		}, actorToken)
		require.EqualValues(t, 1, getActivityUnreadCount(t, readerToken),
			"a new event after the watermark is unread again")
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

	t.Run("seq zero or negative is rejected with 400", func(t *testing.T) {
		resetDB(t)
		_, token := addUser(t, users.NewUserRequest{Username: "marker", Password: "pass"})

		for _, seq := range []int64{0, -1} {
			resp := markActivityReadResponse(t, token, seq)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"seq=%d must be rejected rather than silently accepted", seq)
			resp.Body.Close()
		}
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

	off := httptest.NewServer(server.NewServerWithProvider(testStore, newFakeTitleProvider(), "test-secret"))
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
