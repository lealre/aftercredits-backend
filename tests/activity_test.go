package tests

import (
	"fmt"
	"testing"

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

	t.Run("a failed request records nothing", func(t *testing.T) {
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
