package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddComment(t *testing.T) {
	resetDB(t)

	// ======================================================================
	// 		TEST SETUP - ADDING COMMENTS
	// ======================================================================

	// Create a new user
	user, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add titles to database
	titles := loadTitlesFixture(t)
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	allTitles := append(titles, tvSeriesTitles...)
	seedTitles(t, allTitles)
	expectedMovieTitle := titles[0]
	expectedTVSeriesTitle := tvSeriesTitles[0]
	expectedMovieTitleNotIngroup := titles[1]

	// Add expected title to group
	for _, title := range []imdb.Title{expectedMovieTitle, expectedTVSeriesTitle} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

	// User that is not in the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Expected comment to be used in tests
	expectedComment := "This is a test comment"

	// ======================================================================
	// 		TEST ADDING COMMENTS - MOVIES
	// ======================================================================

	t.Run("Adding a comment sucessfully for a movie", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusCreated, respNewComment.StatusCode)

		var respNewCommentBody comments.Comment
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Equal(t, user.Id, respNewCommentBody.UserId)
		require.Equal(t, expectedMovieTitle.ID, respNewCommentBody.TitleId)
		require.Equal(t, expectedComment, *respNewCommentBody.Comment)
		require.NotEmpty(t, respNewCommentBody.CreatedAt)
		require.Equal(t, respNewCommentBody.CreatedAt, respNewCommentBody.UpdatedAt)

		// Database assertion
		commentDb := getCommentFromDB(t, respNewCommentBody.Id)
		require.Equal(t, user.Id, commentDb.UserId)
		require.Equal(t, expectedMovieTitle.ID, commentDb.TitleId)
		require.NotNil(t, commentDb.Comment)
		require.Equal(t, commentDb.Comment, &expectedComment)
		require.NotEmpty(t, commentDb.CreatedAt)
		require.Equal(t, commentDb.CreatedAt, commentDb.UpdatedAt)
	})

	t.Run("Adding a comment for a movie with a empty comment should return 400", func(t *testing.T) {
		emptyComment := "  "
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Comment: emptyComment,
		}
		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrCommentIsNull.Error()[1:])
	})

	t.Run("Adding a comment for a movie from the same user and title should return 409", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusConflict, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrCommentAlreadyExists.Error()[1:])
	})

	t.Run("Adding a comment for a movie title that is not in the group should return 404", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedMovieTitleNotIngroup.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId), respNewCommentBody.ErrorMessage)
	})

	t.Run("Adding a comment for a movie from user that is not in the group should return 404", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenUserNotInGroup)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId), respNewCommentBody.ErrorMessage)
	})

	// ======================================================================
	// 		TEST ADDING COMMENTS - TV SERIES
	// ======================================================================

	t.Run("Adding a comment sucessfully for a tv series season 1", func(t *testing.T) {
		expectedSeason := 1
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: expectedComment,
			Season:  &expectedSeason,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusCreated, respNewComment.StatusCode)

		var respNewCommentBody comments.Comment
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Equal(t, user.Id, respNewCommentBody.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, respNewCommentBody.TitleId)
		require.Empty(t, respNewCommentBody.Comment)
		require.NotNil(t, respNewCommentBody.SeasonsComments)
		seasonComment := (*respNewCommentBody.SeasonsComments)["1"]
		require.Equal(t, expectedComment, seasonComment.Comment)
		require.NotEmpty(t, seasonComment.AddedAt)
		require.NotEmpty(t, seasonComment.UpdatedAt)
		require.Equal(t, seasonComment.AddedAt, seasonComment.UpdatedAt)
		require.NotEmpty(t, respNewCommentBody.CreatedAt)
		require.Equal(t, respNewCommentBody.CreatedAt, respNewCommentBody.UpdatedAt)

		// Database assertion
		commentDb := getCommentFromDB(t, respNewCommentBody.Id)
		require.Equal(t, user.Id, commentDb.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, commentDb.TitleId)
		require.Nil(t, commentDb.Comment)
		require.NotNil(t, commentDb.SeasonsComments)
		seasonCommentDb := (*commentDb.SeasonsComments)["1"]
		require.Equal(t, expectedComment, seasonCommentDb.Comment)
		require.NotEmpty(t, seasonCommentDb.AddedAt)
		require.NotEmpty(t, seasonCommentDb.UpdatedAt)
		require.Equal(t, seasonCommentDb.AddedAt, seasonCommentDb.UpdatedAt)
		require.NotEmpty(t, commentDb.CreatedAt)
		require.Equal(t, commentDb.CreatedAt, commentDb.UpdatedAt)
	})

	t.Run("Adding a comment for a TV series for other season sucessfully", func(t *testing.T) {
		commentTests := []struct {
			season          int
			expectedComment string
		}{
			{season: 2, expectedComment: "Comment for season 2"},
			{season: 3, expectedComment: "Comment for season 3"},
		}

		for i, tt := range commentTests {
			t.Run(fmt.Sprintf("Season %d", tt.season), func(t *testing.T) {
				newComment := comments.NewComment{
					GroupId: group.Id,
					TitleId: expectedTVSeriesTitle.ID,
					Comment: tt.expectedComment,
					Season:  &tt.season,
				}

				respNewComment := addComment(t, newComment, tokenOwnerUser)
				defer respNewComment.Body.Close()
				require.Equal(t, http.StatusCreated, respNewComment.StatusCode)

				var respNewCommentBody comments.Comment
				require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
				require.Equal(t, user.Id, respNewCommentBody.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, respNewCommentBody.TitleId)
				require.Empty(t, respNewCommentBody.Comment)
				require.NotNil(t, respNewCommentBody.SeasonsComments)
				seasonComment := (*respNewCommentBody.SeasonsComments)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedComment, seasonComment.Comment)
				require.NotEmpty(t, seasonComment.AddedAt)
				require.NotEmpty(t, seasonComment.UpdatedAt)
				require.NotEmpty(t, respNewCommentBody.CreatedAt)
				require.NotEqual(t, respNewCommentBody.CreatedAt, respNewCommentBody.UpdatedAt)
				require.True(t, respNewCommentBody.UpdatedAt.After(respNewCommentBody.CreatedAt))

				// Database assertion
				commentDb := getCommentFromDB(t, respNewCommentBody.Id)
				require.Equal(t, user.Id, commentDb.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, commentDb.TitleId)
				require.Nil(t, commentDb.Comment)
				require.NotNil(t, commentDb.SeasonsComments)
				seasonCommentDb := (*commentDb.SeasonsComments)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedComment, seasonCommentDb.Comment)
				require.NotEmpty(t, seasonCommentDb.AddedAt)
				require.NotEmpty(t, seasonCommentDb.UpdatedAt)
				require.NotEmpty(t, commentDb.CreatedAt)
				require.NotEqual(t, commentDb.CreatedAt, commentDb.UpdatedAt)
				require.True(t, commentDb.UpdatedAt.After(commentDb.CreatedAt))
				// Assert that the SeasonsComments map has the correct number of seasons added
				// One from previous test, and the additional season from the current test
				require.NotEmpty(t, commentDb.SeasonsComments)
				require.Equal(t, i+2, len(*commentDb.SeasonsComments))
			})
		}
	})

	t.Run("Adding a comment for a TV series for a season that do not exist should return 404", func(t *testing.T) {
		expectedSeason := 100
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: expectedComment,
			Season:  &expectedSeason,
		}
		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrSeasonDoesNotExist.Error()[1:])
	})

	t.Run("Adding a comment for a TV series for a season that already has a comment should return 409", func(t *testing.T) {
		expectedSeason := 1
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: "Other comment for season 1",
			Season:  &expectedSeason,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusConflict, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrSeasonCommentAlreadyExists.Error()[1:])
	})

	t.Run("Adding a comment for a TV series for a season without a season number should return 400", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: "Other comment for season 1",
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrSeasonRequired.Error()[1:])
	})

	t.Run("Adding a rating for a TV series for a season with a season number less than 1 should return 400", func(t *testing.T) {
		expectedSeason := 0
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: "Other comment for season 1",
			Season:  &expectedSeason,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrInvalidSeasonValue.Error()[1:])
	})

}

func TestGetComments(t *testing.T) {
	resetDB(t)

	// ======================================================================
	// 		TEST SETUP
	// ======================================================================

	// Create a new user
	user, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add titles to database
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)
	expectedTitle := titles[0]
	titleNotIngroup := titles[1]

	// Add expected title to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// User that is not in the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// ======================================================================
	// 		TEST GETTING COMMENTS
	// ======================================================================

	t.Run("Getting comments from a title with no comments should return 200 and an empty array", func(t *testing.T) {
		respComments := getCommentsFromApi(t, group.Id, expectedTitle.ID, tokenOwnerUser)
		defer respComments.Body.Close()
		require.Equal(t, http.StatusOK, respComments.StatusCode)

		var respGetCommentsBody comments.AllCommentsFromTitle
		require.NoError(t, json.NewDecoder(respComments.Body).Decode(&respGetCommentsBody))
		require.Equal(t, 0, len(respGetCommentsBody.Comments))

		// Database assertion
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 0, len(commentDb))
	})

	// Add comment to title as group owner
	expectedComment := "This is a test comment"
	addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTitle.ID,
		Comment: expectedComment,
	}, tokenOwnerUser)

	t.Run("Getting comments from a title sucessfully", func(t *testing.T) {
		respComments := getCommentsFromApi(t, group.Id, expectedTitle.ID, tokenOwnerUser)
		defer respComments.Body.Close()
		require.Equal(t, http.StatusOK, respComments.StatusCode)

		var respGetCommentsBody comments.AllCommentsFromTitle
		require.NoError(t, json.NewDecoder(respComments.Body).Decode(&respGetCommentsBody))
		require.Equal(t, 1, len(respGetCommentsBody.Comments))
		require.Equal(t, user.Id, respGetCommentsBody.Comments[0].UserId)
		require.Equal(t, expectedTitle.ID, respGetCommentsBody.Comments[0].TitleId)
		require.Equal(t, expectedComment, *respGetCommentsBody.Comments[0].Comment)
		require.NotEmpty(t, respGetCommentsBody.Comments[0].CreatedAt)
		require.Equal(t, respGetCommentsBody.Comments[0].CreatedAt, respGetCommentsBody.Comments[0].UpdatedAt)

		// Database assertion
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, user.Id, commentDb[0].UserId)
		require.Equal(t, expectedTitle.ID, commentDb[0].TitleId)
		require.NotNil(t, commentDb[0].Comment)
		require.Equal(t, commentDb[0].Comment, &expectedComment)
		require.NotEmpty(t, commentDb[0].CreatedAt)
		require.Equal(t, commentDb[0].CreatedAt, commentDb[0].UpdatedAt)
	})

	t.Run("Getting comments from a title as user that is not in the group should return 404", func(t *testing.T) {
		respComments := getCommentsFromApi(t, group.Id, expectedTitle.ID, tokenUserNotInGroup)
		defer respComments.Body.Close()
		require.Equal(t, http.StatusNotFound, respComments.StatusCode)

		var respGetCommentsBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respComments.Body).Decode(&respGetCommentsBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedTitle.ID), respGetCommentsBody.ErrorMessage)
	})

	t.Run("Getting comments from a title that is not in the group should return 404", func(t *testing.T) {
		respComments := getCommentsFromApi(t, group.Id, titleNotIngroup.ID, tokenOwnerUser)
		defer respComments.Body.Close()
		require.Equal(t, http.StatusNotFound, respComments.StatusCode)

		var respGetCommentsBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respComments.Body).Decode(&respGetCommentsBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, titleNotIngroup.ID), respGetCommentsBody.ErrorMessage)
	})
}

func TestUpdateComment(t *testing.T) {
	resetDB(t)

	// ======================================================================
	// 		TEST SETUP
	// ======================================================================

	// Create a new user
	user, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add titles to database
	titles := loadTitlesFixture(t)
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	allTitles := append(titles, tvSeriesTitles...)
	seedTitles(t, allTitles)
	expectedMovieTitle := titles[0]
	expectedTVSeriesTitle := tvSeriesTitles[0]
	expectedTVSeriesTitleNotInGroup := tvSeriesTitles[1]
	// titleNotIngroup := titles[1]

	// Add expected title to group
	for _, title := range []imdb.Title{expectedMovieTitle, expectedTVSeriesTitle} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

	// User that is not in the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Add comment to movie title as group owner
	expectedComment := "This is a test comment"
	commentMovie := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedMovieTitle.ID,
		Comment: expectedComment,
	}, tokenOwnerUser)
	defer commentMovie.Body.Close()
	require.Equal(t, http.StatusCreated, commentMovie.StatusCode)
	var commentCreatedMovie comments.Comment
	require.NoError(t, json.NewDecoder(commentMovie.Body).Decode(&commentCreatedMovie))

	// Add comments to TV series title as group owner
	commentTvSeries := make(map[int]comments.Comment)
	for seasonIndex, comment := range []string{"Season 1 comment", "Season 2 comment"} {
		season := seasonIndex + 1
		commentTVSeries := addComment(t, comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: comment,
			Season:  &season,
		}, tokenOwnerUser)
		defer commentTVSeries.Body.Close()
		require.Equal(t, http.StatusCreated, commentTVSeries.StatusCode)
		var commentCreatedTVSeries comments.Comment
		require.NoError(t, json.NewDecoder(commentTVSeries.Body).Decode(&commentCreatedTVSeries))
		commentTvSeries[seasonIndex] = commentCreatedTVSeries
	}

	// Add delay to ensure UpdatedAt will be different from CreatedAt
	time.Sleep(1 * time.Second)

	// ======================================================================
	// 		TEST UPDATING COMMENTS - MOVIE
	// ======================================================================

	t.Run("Updating a comment for a movie sucessfully", func(t *testing.T) {
		expectedUpdatedComment := "This is a test comment updated"
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedMovieTitle.ID, commentCreatedMovie.Id, expectedUpdatedComment, tokenOwnerUser, nil)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusOK, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody comments.Comment
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Equal(t, user.Id, respUpdatedCommentBody.UserId)
		require.Equal(t, expectedMovieTitle.ID, respUpdatedCommentBody.TitleId)
		require.Equal(t, expectedUpdatedComment, *respUpdatedCommentBody.Comment)
		require.NotEmpty(t, respUpdatedCommentBody.CreatedAt)
		require.NotEqual(t, respUpdatedCommentBody.CreatedAt, respUpdatedCommentBody.UpdatedAt)
		require.True(t, respUpdatedCommentBody.UpdatedAt.After(respUpdatedCommentBody.CreatedAt))

		// Database assertion
		commentDb := getCommentFromDB(t, respUpdatedCommentBody.Id)
		require.Equal(t, user.Id, commentDb.UserId)
		require.Equal(t, expectedMovieTitle.ID, commentDb.TitleId)
		require.NotNil(t, commentDb.Comment)
		require.Equal(t, commentDb.Comment, &expectedUpdatedComment)
		require.NotEmpty(t, commentDb.CreatedAt)
		require.NotEqual(t, commentDb.CreatedAt, commentDb.UpdatedAt)
		require.True(t, commentDb.UpdatedAt.After(commentDb.CreatedAt))
	})

	t.Run("Updating a comment for a movie that is not from the user should return 404", func(t *testing.T) {
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedMovieTitle.ID, commentCreatedMovie.Id, "This is a test comment updated", tokenUserNotInGroup, nil)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedMovieTitle.ID))
	})

	t.Run("Updating a comment for a movie with a empty comment should return 400", func(t *testing.T) {
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedMovieTitle.ID, commentCreatedMovie.Id, "   ", tokenOwnerUser, nil)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, comments.ErrCommentIsNull.Error()[1:])
	})

	// ======================================================================
	// 		TEST UPDATING COMMENTS - TV SERIES
	// ======================================================================

	t.Run("Updating a comment for a TV series season sucessfully", func(t *testing.T) {
		commentTests := []struct {
			season          int
			expectedComment string
		}{
			{season: 1, expectedComment: "Season 1 comment updated"},
			{season: 2, expectedComment: "Season 2 comment updated"},
		}

		for _, tt := range commentTests {
			t.Run(fmt.Sprintf("Season %d", tt.season), func(t *testing.T) {
				respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTVSeriesTitle.ID, commentTvSeries[tt.season-1].Id, tt.expectedComment, tokenOwnerUser, &tt.season)
				defer respUpdatedComment.Body.Close()
				// body, err := io.ReadAll(respUpdatedComment.Body)
				// require.NoError(t, err)
				// fmt.Println(string(body))
				require.Equal(t, http.StatusOK, respUpdatedComment.StatusCode)

				var respUpdatedCommentBody comments.Comment
				require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
				require.Equal(t, user.Id, respUpdatedCommentBody.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, respUpdatedCommentBody.TitleId)
				seasonComment := (*respUpdatedCommentBody.SeasonsComments)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedComment, seasonComment.Comment)
				require.NotEmpty(t, seasonComment.AddedAt)
				require.NotEmpty(t, seasonComment.UpdatedAt)
				require.True(t, seasonComment.UpdatedAt.After(seasonComment.AddedAt) || seasonComment.UpdatedAt.Equal(seasonComment.AddedAt))
				require.NotEmpty(t, respUpdatedCommentBody.CreatedAt)
				require.NotEqual(t, respUpdatedCommentBody.CreatedAt, respUpdatedCommentBody.UpdatedAt)
				require.True(t, respUpdatedCommentBody.UpdatedAt.After(respUpdatedCommentBody.CreatedAt))

				// Database assertion
				commentDb := getCommentFromDB(t, respUpdatedCommentBody.Id)
				require.Equal(t, user.Id, commentDb.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, commentDb.TitleId)
				seasonCommentDb := (*commentDb.SeasonsComments)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedComment, seasonCommentDb.Comment)
				require.NotEmpty(t, seasonCommentDb.AddedAt)
				require.NotEmpty(t, seasonCommentDb.UpdatedAt)
				require.True(t, seasonCommentDb.UpdatedAt.After(seasonCommentDb.AddedAt) || seasonCommentDb.UpdatedAt.Equal(seasonCommentDb.AddedAt))
				require.NotEmpty(t, commentDb.CreatedAt)
				require.NotEqual(t, commentDb.CreatedAt, commentDb.UpdatedAt)
				require.True(t, commentDb.UpdatedAt.After(commentDb.CreatedAt))
			})
		}
	})

	t.Run("Updating a TV series season comment that does not exist should return 404", func(t *testing.T) {
		season := 1
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTVSeriesTitleNotInGroup.ID, commentTvSeries[0].Id, "This is a test comment updated", tokenUserNotInGroup, &season)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedTVSeriesTitleNotInGroup.ID))
	})

	t.Run("Updating a TV series season comment whithout a season number should return 400", func(t *testing.T) {
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTVSeriesTitle.ID, commentTvSeries[0].Id, "This is a test comment updated", tokenOwnerUser, nil)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, comments.ErrSeasonRequired.Error()[1:])
	})
}

func TestDeleteComment(t *testing.T) {
	resetDB(t)

	// ======================================================================
	// 		TEST SETUP
	// ======================================================================

	// Create a new user (group owner)
	_, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a new user (group user that will be added to the group)
	userFromGroup, tokenUserFromGroup := addUser(t, users.NewUserRequest{
		Username: "testname2",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add user to group
	addUserToGroup(t, groups.AddUserToGroupRequest{
		UserId: userFromGroup.Id,
	}, group.Id, tokenOwnerUser)

	// Add titles to database
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)
	expectedTitle := titles[0]
	// titleNotIngroup := titles[1]

	// Add expected title to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// User that is not in the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Add comment to title as group owner
	expectedOwnerComment := "This is a test comment"
	comment := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTitle.ID,
		Comment: expectedOwnerComment,
	}, tokenOwnerUser)
	defer comment.Body.Close()
	require.Equal(t, http.StatusCreated, comment.StatusCode)
	var commentCreatedOwner comments.Comment
	require.NoError(t, json.NewDecoder(comment.Body).Decode(&commentCreatedOwner))

	// Add comment to title as group user
	expectedGroupComment := "This is a test comment"
	commentFromGroup := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTitle.ID,
		Comment: expectedGroupComment,
	}, tokenUserFromGroup)
	defer commentFromGroup.Body.Close()
	require.Equal(t, http.StatusCreated, commentFromGroup.StatusCode)
	var commentCreatedGroup comments.Comment
	require.NoError(t, json.NewDecoder(commentFromGroup.Body).Decode(&commentCreatedGroup))

	// ======================================================================
	// 		TEST DELETING COMMENTS
	// ======================================================================

	t.Run("Deleting a comment sucessfully", func(t *testing.T) {
		// Delete owner's comment
		respDeletedComment := deleteCommentFromApi(t, group.Id, expectedTitle.ID, commentCreatedOwner.Id, tokenOwnerUser)
		defer respDeletedComment.Body.Close()
		require.Equal(t, http.StatusOK, respDeletedComment.StatusCode)

		var respDeletedCommentBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeletedComment.Body).Decode(&respDeletedCommentBody))
		require.Equal(t, fmt.Sprintf("Comment with id %s deleted successfully", commentCreatedOwner.Id), respDeletedCommentBody.Message)

		// Database assertion - Should remain just the comment from the group user, not the owner's comment
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, userFromGroup.Id, commentDb[0].UserId)
		require.Equal(t, expectedTitle.ID, commentDb[0].TitleId)
		require.NotNil(t, commentDb[0].Comment)
		require.Equal(t, *commentDb[0].Comment, expectedGroupComment)
		require.NotEmpty(t, commentDb[0].CreatedAt)
		require.Equal(t, commentDb[0].CreatedAt, commentDb[0].UpdatedAt)
	})

	t.Run("Deleting a comment that is not from the user but is from the group should return 404", func(t *testing.T) {
		// Try to delete the comment from the group user as the owner
		respDeletedComment := deleteCommentFromApi(t, group.Id, expectedTitle.ID, commentCreatedGroup.Id, tokenOwnerUser)
		defer respDeletedComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeletedComment.StatusCode)
		var respDeletedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeletedComment.Body).Decode(&respDeletedCommentBody))
		require.Contains(t, respDeletedCommentBody.ErrorMessage, fmt.Sprintf("Comment with id %s not found", commentCreatedGroup.Id))

		// Database assertion - Should remain just the comment from the group user, not the owner's comment
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, userFromGroup.Id, commentDb[0].UserId)
		require.Equal(t, expectedTitle.ID, commentDb[0].TitleId)
		require.NotNil(t, commentDb[0].Comment)
		require.Equal(t, *commentDb[0].Comment, expectedGroupComment)
		require.NotEmpty(t, commentDb[0].CreatedAt)
		require.Equal(t, commentDb[0].CreatedAt, commentDb[0].UpdatedAt)
	})

	t.Run("Deleting a comment that is not from the user and not from the group should return 404", func(t *testing.T) {
		// Try to delete the comment from the group user as a user that is not in the group
		respDeletedComment := deleteCommentFromApi(t, group.Id, expectedTitle.ID, commentCreatedGroup.Id, tokenUserNotInGroup)
		defer respDeletedComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeletedComment.StatusCode)
		var respDeletedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeletedComment.Body).Decode(&respDeletedCommentBody))
		require.Contains(t, respDeletedCommentBody.ErrorMessage, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedTitle.ID))

		// Database assertion - Should remain just the comment from the group user, not the owner's comment
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, userFromGroup.Id, commentDb[0].UserId)
		require.Equal(t, expectedTitle.ID, commentDb[0].TitleId)
		require.NotNil(t, commentDb[0].Comment)
		require.Equal(t, *commentDb[0].Comment, expectedGroupComment)
		require.NotEmpty(t, commentDb[0].CreatedAt)
		require.Equal(t, commentDb[0].CreatedAt, commentDb[0].UpdatedAt)
	})
}

func TestDeleteCommentSeason(t *testing.T) {
	resetDB(t)

	// ======================================================================
	// 		TEST SETUP
	// ======================================================================

	// Create a new user (group owner)
	_, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a new user (group user that will be added to the group)
	userFromGroup, tokenUserFromGroup := addUser(t, users.NewUserRequest{
		Username: "testname2",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add user to group
	addUserToGroup(t, groups.AddUserToGroupRequest{
		UserId: userFromGroup.Id,
	}, group.Id, tokenOwnerUser)

	// User that is not in the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Seed titles
	movieTitles := loadTitlesFixture(t)
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	allTitles := append(movieTitles, tvSeriesTitles...)
	seedTitles(t, allTitles)

	expectedTVSeriesTitle := tvSeriesTitles[0]
	expectedTVSeriesTitle2 := tvSeriesTitles[1]

	// Add tv series titles to group
	for _, title := range []imdb.Title{expectedTVSeriesTitle, expectedTVSeriesTitle2} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

	// Create a TV series comment with seasons 1 and 2
	season1 := 1
	season2 := 2
	respSeason1 := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTVSeriesTitle.ID,
		Comment: "Comment for season 1",
		Season:  &season1,
	}, tokenOwnerUser)
	defer respSeason1.Body.Close()
	require.Equal(t, http.StatusCreated, respSeason1.StatusCode)
	var createdSeason1 comments.Comment
	require.NoError(t, json.NewDecoder(respSeason1.Body).Decode(&createdSeason1))

	respSeason2 := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTVSeriesTitle.ID,
		Comment: "Comment for season 2",
		Season:  &season2,
	}, tokenOwnerUser)
	defer respSeason2.Body.Close()
	require.Equal(t, http.StatusCreated, respSeason2.StatusCode)
	var createdSeason2 comments.Comment
	require.NoError(t, json.NewDecoder(respSeason2.Body).Decode(&createdSeason2))
	require.Equal(t, createdSeason1.Id, createdSeason2.Id, "Expected same comment id for multiple seasons of the same TV series")

	commentId := createdSeason1.Id

	// ======================================================================
	// 		TEST DELETING SEASON COMMENTS
	// ======================================================================

	t.Run("Deleting a season comment successfully", func(t *testing.T) {
		respDeleted := deleteCommentSeasonFromApi(t, group.Id, expectedTVSeriesTitle.ID, commentId, tokenOwnerUser, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Season %d from comment %s deleted successfully", season1, commentId), respBody.Message)

		// DB assertion: comment still exists, but season 1 was removed
		commentDb := getCommentFromDB(t, commentId)
		require.NotNil(t, commentDb.SeasonsComments)
		_, ok := (*commentDb.SeasonsComments)["1"]
		require.False(t, ok, "Expected season 1 to be deleted from SeasonsComments")
		season2CommentDb := (*commentDb.SeasonsComments)["2"]
		require.Equal(t, "Comment for season 2", season2CommentDb.Comment)
		require.NotEmpty(t, season2CommentDb.AddedAt)
		require.NotEmpty(t, season2CommentDb.UpdatedAt)

		// API assertion: returned comments should only contain season 2
		respGet := getCommentsFromApi(t, group.Id, expectedTVSeriesTitle.ID, tokenOwnerUser)
		defer respGet.Body.Close()
		require.Equal(t, http.StatusOK, respGet.StatusCode)

		var allComments comments.AllCommentsFromTitle
		require.NoError(t, json.NewDecoder(respGet.Body).Decode(&allComments))
		require.Len(t, allComments.Comments, 1)
		require.NotNil(t, allComments.Comments[0].SeasonsComments)
		_, ok = (*allComments.Comments[0].SeasonsComments)["1"]
		require.False(t, ok, "Expected season 1 to be deleted from API response SeasonsComments")
		season2Comment := (*allComments.Comments[0].SeasonsComments)["2"]
		require.Equal(t, "Comment for season 2", season2Comment.Comment)
		require.NotEmpty(t, season2Comment.AddedAt)
		require.NotEmpty(t, season2Comment.UpdatedAt)
	})

	t.Run("Deleting last season should delete the whole comment document", func(t *testing.T) {
		onlySeason := 1
		respOnly := addComment(t, comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle2.ID,
			Comment: "Only season comment",
			Season:  &onlySeason,
		}, tokenOwnerUser)
		defer respOnly.Body.Close()
		require.Equal(t, http.StatusCreated, respOnly.StatusCode)
		var createdOnly comments.Comment
		require.NoError(t, json.NewDecoder(respOnly.Body).Decode(&createdOnly))

		respDeleted := deleteCommentSeasonFromApi(t, group.Id, expectedTVSeriesTitle2.ID, createdOnly.Id, tokenOwnerUser, onlySeason)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		// DB assertion: no comments for this title should remain
		commentsDb := getCommentsFromDB(t, expectedTVSeriesTitle2.ID)
		require.Empty(t, commentsDb)

		// API assertion: should return empty comments list
		respGet := getCommentsFromApi(t, group.Id, expectedTVSeriesTitle2.ID, tokenOwnerUser)
		defer respGet.Body.Close()
		require.Equal(t, http.StatusOK, respGet.StatusCode)
		var allComments comments.AllCommentsFromTitle
		require.NoError(t, json.NewDecoder(respGet.Body).Decode(&allComments))
		require.Empty(t, allComments.Comments)
	})

	t.Run("Deleting a season comment not being from group should return 404", func(t *testing.T) {
		respDeleted := deleteCommentSeasonFromApi(t, group.Id, expectedTVSeriesTitle.ID, commentId, tokenUserNotInGroup, season2)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respDeletedBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respDeletedBody))
		require.Contains(t, respDeletedBody.ErrorMessage, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedTVSeriesTitle.ID))
	})

	t.Run("Deleting a season comment with invalid season should return 400", func(t *testing.T) {
		respDeleted := deleteCommentSeasonFromApi(t, group.Id, expectedTVSeriesTitle.ID, commentId, tokenOwnerUser, 0)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusBadRequest, respDeleted.StatusCode)

		var respDeletedBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respDeletedBody))
		require.Contains(t, respDeletedBody.ErrorMessage, comments.ErrInvalidSeasonValue.Error()[1:])
	})

	t.Run("Deleting a season comment that does not exist should return 404", func(t *testing.T) {
		// Add a comment for the group user on season 1
		resp := addComment(t, comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: "User season 1",
			Season:  &season1,
		}, tokenUserFromGroup)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var created comments.Comment
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

		// Try to delete season 2 which exists in the title but not in the comment map for this user
		respDeleted := deleteCommentSeasonFromApi(t, group.Id, expectedTVSeriesTitle.ID, created.Id, tokenUserFromGroup, season2)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respDeletedBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respDeletedBody))
		require.Contains(t, respDeletedBody.ErrorMessage, comments.ErrCommentNotFound.Error()[1:])
	})
}
