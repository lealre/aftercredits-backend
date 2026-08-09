package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/models"
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
	for _, title := range []models.Title{expectedMovieTitle, expectedTVSeriesTitle} {
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
	for _, title := range []models.Title{expectedMovieTitle, expectedTVSeriesTitle} {
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
	for _, title := range []models.Title{expectedTVSeriesTitle, expectedTVSeriesTitle2} {
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

func TestGetCommentsForTVSeries(t *testing.T) {
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

	// Add a TV series title to the database and to the group
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	seedTitles(t, tvSeriesTitles)
	expectedTVSeriesTitle := tvSeriesTitles[0]

	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTVSeriesTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// Add per-season comments for the TV series
	commentTests := []struct {
		season  int
		comment string
	}{
		{season: 1, comment: "Season 1 comment"},
		{season: 2, comment: "Season 2 comment"},
	}

	for _, tt := range commentTests {
		season := tt.season
		respComment := addComment(t, comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Comment: tt.comment,
			Season:  &season,
		}, tokenOwnerUser)
		defer respComment.Body.Close()
		require.Equal(t, http.StatusCreated, respComment.StatusCode)
	}

	// ======================================================================
	// 		TEST GETTING COMMENTS FOR A TV SERIES
	// ======================================================================

	t.Run("Get comments for a TV series", func(t *testing.T) {
		respComments := getCommentsFromApi(t, group.Id, expectedTVSeriesTitle.ID, tokenOwnerUser)
		defer respComments.Body.Close()
		require.Equal(t, http.StatusOK, respComments.StatusCode)

		var respGetCommentsBody comments.AllCommentsFromTitle
		require.NoError(t, json.NewDecoder(respComments.Body).Decode(&respGetCommentsBody))
		require.Equal(t, 1, len(respGetCommentsBody.Comments), "Expected a single comment document for the TV series (one per user, holding all seasons)")

		commentResp := respGetCommentsBody.Comments[0]
		require.Equal(t, user.Id, commentResp.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, commentResp.TitleId)
		require.Nil(t, commentResp.Comment, "Expected top-level Comment to be nil for a TV series comment")
		require.NotNil(t, commentResp.SeasonsComments)
		require.Equal(t, len(commentTests), len(*commentResp.SeasonsComments))

		for _, tt := range commentTests {
			seasonComment, exists := (*commentResp.SeasonsComments)[strconv.Itoa(tt.season)]
			require.True(t, exists, "Expected season %d comment to exist in response", tt.season)
			require.Equal(t, tt.comment, seasonComment.Comment)
			require.NotEmpty(t, seasonComment.AddedAt)
			require.NotEmpty(t, seasonComment.UpdatedAt)
		}

		// Database assertion
		commentDb := getCommentsFromDB(t, expectedTVSeriesTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, user.Id, commentDb[0].UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, commentDb[0].TitleId)
		require.Nil(t, commentDb[0].Comment)
		require.NotNil(t, commentDb[0].SeasonsComments)
		require.Equal(t, len(commentTests), len(*commentDb[0].SeasonsComments))

		for _, tt := range commentTests {
			seasonCommentDb, exists := (*commentDb[0].SeasonsComments)[strconv.Itoa(tt.season)]
			require.True(t, exists, "Expected season %d comment to exist in database", tt.season)
			require.Equal(t, tt.comment, seasonCommentDb.Comment)
			require.NotEmpty(t, seasonCommentDb.AddedAt)
			require.NotEmpty(t, seasonCommentDb.UpdatedAt)
		}
	})
}

// TestCommentsAreGroupScoped covers the invariant that a comment's identity is
// (user, title, group), exercised end to end through the API.
//
// The read path used to filter by the requesting group's member list rather
// than by the group itself, so a comment left by a shared member in another
// group surfaced in both. The leak regression subtest below is built around
// exactly that shared member.
func TestCommentsAreGroupScoped(t *testing.T) {
	t.Run("The same user comments on the same title in two groups and both comments persist", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupA := "comment in group A"
		textGroupB := "comment in group B"

		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, textGroupA, nil, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textGroupB, nil, world.token)

		require.NotEqual(t, commentGroupA.Id, commentGroupB.Id,
			"Expected commenting on the same title in a second group to create a distinct comment")
		require.Equal(t, world.groupA.Id, commentGroupA.GroupId, "Expected the comment created in group A to report group A")
		require.Equal(t, world.groupB.Id, commentGroupB.GroupId, "Expected the comment created in group B to report group B")

		// Database assertion: two rows, one per group, each keeping its own text.
		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.Equal(t, world.groupA.Id, commentGroupADb.GroupId, "Expected group A's stored comment to carry group A's id")
		require.Equal(t, &textGroupA, commentGroupADb.Comment, "Expected group A's stored comment to keep group A's text")
		require.Equal(t, world.user.Id, commentGroupADb.UserId, "Expected group A's stored comment to belong to the commenting user")

		commentGroupBDb := getCommentFromDB(t, commentGroupB.Id)
		require.Equal(t, world.groupB.Id, commentGroupBDb.GroupId, "Expected group B's stored comment to carry group B's id")
		require.Equal(t, &textGroupB, commentGroupBDb.Comment, "Expected group B's stored comment to keep group B's text")

		require.Len(t, getCommentsFromDB(t, world.movie.ID), 2,
			"Expected exactly one stored comment per group for this user and title")
	})

	t.Run("The comments read path returns only the requesting group's comments", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupA := "comment in group A"
		textGroupB := "comment in group B"
		textOtherUserGroupB := "comment from the other member in group B"

		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, textGroupA, nil, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textGroupB, nil, world.token)
		commentOtherUserGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textOtherUserGroupB, nil, world.otherToken)

		// Sanity check first: all three comments really are stored, so a short
		// group A list below cannot be mistaken for "nothing was written".
		require.Len(t, getCommentsFromDB(t, world.movie.ID), 3,
			"Expected all three comments on this title to be stored before asserting what each group sees")

		commentsGroupA := getCommentsForGroupTitle(t, world.groupA.Id, world.movie.ID, world.token)
		require.Len(t, commentsGroupA, 1,
			"Expected group A to return only the comment left in group A, not the same user's comment in group B")
		require.Equal(t, commentGroupA.Id, commentsGroupA[0].Id, "Expected group A to return the comment created in group A")
		require.Equal(t, world.groupA.Id, commentsGroupA[0].GroupId,
			"Expected every comment returned for group A to be attributed to group A")
		require.Equal(t, textGroupA, *commentsGroupA[0].Comment, "Expected group A to return group A's comment text")

		commentsGroupB := getCommentsForGroupTitle(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, commentsGroupB, 2,
			"Expected group B to return both of its own members' comments and nothing from group A")

		returnedIds := []string{}
		returnedTexts := []string{}
		for _, comment := range commentsGroupB {
			require.Equal(t, world.groupB.Id, comment.GroupId,
				"Expected every comment returned for group B to be attributed to group B")
			require.NotNil(t, comment.Comment, "Expected every movie comment returned for group B to carry text")
			returnedIds = append(returnedIds, comment.Id)
			returnedTexts = append(returnedTexts, *comment.Comment)
		}
		require.ElementsMatch(t, []string{commentGroupB.Id, commentOtherUserGroupB.Id}, returnedIds,
			"Expected group B to return exactly the two comments left in group B")
		require.ElementsMatch(t, []string{textGroupB, textOtherUserGroupB}, returnedTexts,
			"Expected group B to return group B's comment texts")
	})

	t.Run("Commenting on a title in a second group is allowed but commenting twice in the same group still returns 409", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		_ = addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, "comment in group A", nil, world.token)

		// A comment in another group is a different fact, not a duplicate.
		respSecondGroup := addComment(t, comments.NewComment{
			GroupId: world.groupB.Id,
			TitleId: world.movie.ID,
			Comment: "comment in group B",
		}, world.token)
		defer respSecondGroup.Body.Close()
		require.Equal(t, http.StatusCreated, respSecondGroup.StatusCode,
			"Expected commenting on a title already commented in another group to be allowed")

		// A second comment in the same group still is a duplicate.
		respSameGroup := addComment(t, comments.NewComment{
			GroupId: world.groupA.Id,
			TitleId: world.movie.ID,
			Comment: "second comment in group A",
		}, world.token)
		defer respSameGroup.Body.Close()
		require.Equal(t, http.StatusConflict, respSameGroup.StatusCode,
			"Expected a second comment on the same title in the same group to still conflict")

		var respSameGroupBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respSameGroup.Body).Decode(&respSameGroupBody),
			"error decoding the conflict response body")
		require.Contains(t, respSameGroupBody.ErrorMessage, comments.ErrCommentAlreadyExists.Error()[1:],
			"Expected the same-group duplicate to report the comment-already-exists error")

		require.Len(t, getCommentsFromDB(t, world.movie.ID), 2,
			"Expected the rejected duplicate to leave exactly the two group-scoped comments")
	})

	t.Run("Updating a comment leaves the other group's comment untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupB := "comment in group B"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, "comment in group A", nil, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textGroupB, nil, world.token)

		updatedText := "comment in group A, edited"
		respUpdated := updateCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupA.Id, updatedText, world.token, nil)
		defer respUpdated.Body.Close()
		require.Equal(t, http.StatusOK, respUpdated.StatusCode, "Expected group A's comment to be updated")

		var respUpdatedBody comments.Comment
		require.NoError(t, json.NewDecoder(respUpdated.Body).Decode(&respUpdatedBody), "error decoding the updated comment")
		require.Equal(t, updatedText, *respUpdatedBody.Comment, "Expected the updated comment to carry the new text")
		require.Equal(t, world.groupA.Id, respUpdatedBody.GroupId, "Expected updating a comment not to move it to another group")

		commentGroupBDb := getCommentFromDB(t, commentGroupB.Id)
		require.Equal(t, &textGroupB, commentGroupBDb.Comment,
			"Expected group B's comment to be untouched by a PATCH aimed at group A's comment")
		require.Equal(t, world.groupB.Id, commentGroupBDb.GroupId, "Expected group B's comment to stay attributed to group B")

		commentsGroupB := getCommentsForGroupTitle(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, commentsGroupB, 1, "Expected group B to still return exactly its own comment")
		require.Equal(t, textGroupB, *commentsGroupB[0].Comment, "Expected group B to still return its original text")
	})

	t.Run("Deleting a comment leaves the other group's comment untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupB := "comment in group B"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, "comment in group A", nil, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textGroupB, nil, world.token)

		respDeleted := deleteCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupA.Id, world.token)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode, "Expected group A's comment to be deleted")

		commentsDb := getCommentsFromDB(t, world.movie.ID)
		require.Len(t, commentsDb, 1, "Expected group B's comment to survive a DELETE aimed at group A's comment")
		require.Equal(t, commentGroupB.Id, commentsDb[0].Id, "Expected the surviving comment to be group B's")
		require.Equal(t, world.groupB.Id, commentsDb[0].GroupId, "Expected the surviving comment to be attributed to group B")

		require.Empty(t, getCommentsForGroupTitle(t, world.groupA.Id, world.movie.ID, world.token),
			"Expected group A to return no comments after deleting its only one")

		commentsGroupB := getCommentsForGroupTitle(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, commentsGroupB, 1, "Expected group B to still return its own comment")
		require.Equal(t, textGroupB, *commentsGroupB[0].Comment, "Expected group B's text to be unchanged")
	})

	t.Run("Addressing group A's URL with group B's comment id is rejected and changes nothing", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		// The caller is a member of both groups and both hold the movie, so the
		// route guard passes for either group. Only the (title, user, group) key
		// distinguishes the two rows: a client resolving comment ids from a stale
		// list must not be able to edit or delete group B's comment through group
		// A's URL.
		textGroupA := "comment in group A"
		textGroupB := "comment in group B"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, textGroupA, nil, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.movie.ID, textGroupB, nil, world.token)

		respUpdate := updateCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupB.Id, "edited through group A", world.token, nil)
		defer respUpdate.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdate.StatusCode,
			"Expected a PATCH under group A's URL carrying group B's comment id to return 404")

		respDelete := deleteCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupB.Id, world.token)
		defer respDelete.Body.Close()
		require.Equal(t, http.StatusNotFound, respDelete.StatusCode,
			"Expected a DELETE under group A's URL carrying group B's comment id to return 404")

		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.Equal(t, &textGroupA, commentGroupADb.Comment,
			"Expected group A's comment to be untouched by the rejected cross-group requests")
		commentGroupBDb := getCommentFromDB(t, commentGroupB.Id)
		require.Equal(t, &textGroupB, commentGroupBDb.Comment,
			"Expected group B's comment to be untouched by requests addressed to group A")
		require.Equal(t, world.groupB.Id, commentGroupBDb.GroupId,
			"Expected group B's comment to stay attributed to group B")

		require.Len(t, getCommentsFromDB(t, world.movie.ID), 2,
			"Expected both group-scoped comments to survive the rejected cross-group requests")
	})

	t.Run("A member of another group still gets 404 on the group-scoped comment routes", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		// otherUser is a member of group B only. Group A holds the title and a
		// comment, but none of that is visible or writable from outside group A.
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.movie.ID, "comment in group A", nil, world.token)
		expectedMessage := fmt.Sprintf("Group %s do not have title %s or do not exist.", world.groupA.Id, world.movie.ID)

		respGet := getCommentsFromApi(t, world.groupA.Id, world.movie.ID, world.otherToken)
		defer respGet.Body.Close()
		require.Equal(t, http.StatusNotFound, respGet.StatusCode,
			"Expected reading group A's comments as a non-member to return 404")
		var respGetBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respGet.Body).Decode(&respGetBody), "error decoding the read 404 body")
		require.Equal(t, expectedMessage, respGetBody.ErrorMessage,
			"Expected the non-member read to keep reporting the group-does-not-have-title message")

		respAdd := addComment(t, comments.NewComment{
			GroupId: world.groupA.Id,
			TitleId: world.movie.ID,
			Comment: "comment from a non-member",
		}, world.otherToken)
		defer respAdd.Body.Close()
		require.Equal(t, http.StatusNotFound, respAdd.StatusCode,
			"Expected commenting into group A as a non-member to return 404")

		respUpdate := updateCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupA.Id, "edited by a non-member", world.otherToken, nil)
		defer respUpdate.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdate.StatusCode,
			"Expected updating group A's comment as a non-member to return 404")

		respDelete := deleteCommentFromApi(t, world.groupA.Id, world.movie.ID, commentGroupA.Id, world.otherToken)
		defer respDelete.Body.Close()
		require.Equal(t, http.StatusNotFound, respDelete.StatusCode,
			"Expected deleting group A's comment as a non-member to return 404")

		// Nothing the non-member attempted may have taken effect.
		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.Equal(t, "comment in group A", *commentGroupADb.Comment,
			"Expected group A's comment to be untouched by the rejected non-member requests")
		require.Len(t, getCommentsFromDB(t, world.movie.ID), 1,
			"Expected the rejected non-member insert to leave exactly group A's comment")
	})
}

// TestCommentsForTVSeriesAreGroupScoped exercises the per-season comment flow
// with two groups in play. Both the add and the update path resolve an existing
// comment by (user, title, group); without the group predicate that lookup
// matches one row per group and silently resolves to an arbitrary one.
func TestCommentsForTVSeriesAreGroupScoped(t *testing.T) {
	season1 := 1
	season2 := 2

	t.Run("Season comments for the same series stay independent in each group", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupASeason1 := "group A season 1"
		textGroupBSeason1 := "group B season 1"
		textGroupBSeason2 := "group B season 2"

		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.tvSeries.ID, textGroupASeason1, &season1, world.token)
		commentGroupBFirst := addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, textGroupBSeason1, &season1, world.token)
		commentGroupBSecond := addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, textGroupBSeason2, &season2, world.token)

		require.NotEqual(t, commentGroupA.Id, commentGroupBFirst.Id,
			"Expected the series comment in each group to be a distinct row")
		require.Equal(t, commentGroupBFirst.Id, commentGroupBSecond.Id,
			"Expected both group B seasons to land on the same group B comment")

		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.Equal(t, world.groupA.Id, commentGroupADb.GroupId, "Expected group A's series comment to carry group A's id")
		require.NotNil(t, commentGroupADb.SeasonsComments, "Expected group A's series comment to hold season comments")
		require.Len(t, *commentGroupADb.SeasonsComments, 1,
			"Expected group A's series comment to hold only the season commented in group A")
		require.Equal(t, textGroupASeason1, (*commentGroupADb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group A's season 1 comment to keep group A's text")

		commentGroupBDb := getCommentFromDB(t, commentGroupBFirst.Id)
		require.Equal(t, world.groupB.Id, commentGroupBDb.GroupId, "Expected group B's series comment to carry group B's id")
		require.NotNil(t, commentGroupBDb.SeasonsComments, "Expected group B's series comment to hold season comments")
		require.Len(t, *commentGroupBDb.SeasonsComments, 2,
			"Expected group B's series comment to hold both seasons commented in group B")
		require.Equal(t, textGroupBSeason1, (*commentGroupBDb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group B's season 1 comment to keep group B's text, not group A's")

		commentsGroupA := getCommentsForGroupTitle(t, world.groupA.Id, world.tvSeries.ID, world.token)
		require.Len(t, commentsGroupA, 1, "Expected group A to return only its own series comment")
		require.Equal(t, world.groupA.Id, commentsGroupA[0].GroupId, "Expected group A's series comment to be attributed to group A")
		require.NotNil(t, commentsGroupA[0].SeasonsComments, "Expected group A's returned series comment to carry its seasons")
		require.Len(t, *commentsGroupA[0].SeasonsComments, 1,
			"Expected group A's returned series comment to carry only the season commented in group A")

		commentsGroupB := getCommentsForGroupTitle(t, world.groupB.Id, world.tvSeries.ID, world.token)
		require.Len(t, commentsGroupB, 1, "Expected group B to return only its own series comment")
		require.Equal(t, world.groupB.Id, commentsGroupB[0].GroupId, "Expected group B's series comment to be attributed to group B")
		require.NotNil(t, commentsGroupB[0].SeasonsComments, "Expected group B's returned series comment to carry its seasons")
		require.Len(t, *commentsGroupB[0].SeasonsComments, 2,
			"Expected group B's returned series comment to carry both seasons commented in group B")
	})

	t.Run("Updating a season comment resolves the requesting group's comment", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		// Deliberately different seasons per group: a lookup that ignored the
		// group could resolve to the other group's comment, which does not hold
		// the season being edited at all.
		textGroupASeason1 := "group A season 1"
		textGroupBSeason2 := "group B season 2"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.tvSeries.ID, textGroupASeason1, &season1, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, textGroupBSeason2, &season2, world.token)

		updatedText := "group B season 2, edited"
		respUpdated := updateCommentFromApi(t, world.groupB.Id, world.tvSeries.ID, commentGroupB.Id, updatedText, world.token, &season2)
		defer respUpdated.Body.Close()
		require.Equal(t, http.StatusOK, respUpdated.StatusCode,
			"Expected group B's season 2 comment to be updated even though group A holds a different season")

		var respUpdatedBody comments.Comment
		require.NoError(t, json.NewDecoder(respUpdated.Body).Decode(&respUpdatedBody), "error decoding the updated season comment")
		require.Equal(t, world.groupB.Id, respUpdatedBody.GroupId, "Expected the updated season comment to stay in group B")
		require.NotNil(t, respUpdatedBody.SeasonsComments, "Expected the updated season comment to carry its seasons")
		require.Equal(t, updatedText, (*respUpdatedBody.SeasonsComments)[strconv.Itoa(season2)].Comment,
			"Expected group B's season 2 text to be the edited one")

		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.NotNil(t, commentGroupADb.SeasonsComments, "Expected group A's series comment to still hold its season")
		require.Equal(t, textGroupASeason1, (*commentGroupADb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group A's season 1 comment to be untouched by a PATCH aimed at group B")
		require.Len(t, *commentGroupADb.SeasonsComments, 1, "Expected group A's seasons to be untouched")
	})

	t.Run("Addressing group A's URL with group B's series comment id is rejected and changes nothing", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		// Both comments hold season 1, so every season check the update path runs
		// passes against either row. Only binding the path's commentId to the
		// group-scoped lookup keeps the write off group B's comment.
		textGroupASeason1 := "group A season 1"
		textGroupBSeason1 := "group B season 1"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.tvSeries.ID, textGroupASeason1, &season1, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, textGroupBSeason1, &season1, world.token)

		respUpdate := updateCommentFromApi(t, world.groupA.Id, world.tvSeries.ID, commentGroupB.Id, "written via group A route", world.token, &season1)
		defer respUpdate.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdate.StatusCode,
			"Expected a season PATCH under group A's URL carrying group B's comment id to return 404")

		respDeleteSeason := deleteCommentSeasonFromApi(t, world.groupA.Id, world.tvSeries.ID, commentGroupB.Id, world.token, season1)
		defer respDeleteSeason.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleteSeason.StatusCode,
			"Expected a season DELETE under group A's URL carrying group B's comment id to return 404")

		respDelete := deleteCommentFromApi(t, world.groupA.Id, world.tvSeries.ID, commentGroupB.Id, world.token)
		defer respDelete.Body.Close()
		require.Equal(t, http.StatusNotFound, respDelete.StatusCode,
			"Expected a DELETE under group A's URL carrying group B's series comment id to return 404")

		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.NotNil(t, commentGroupADb.SeasonsComments, "Expected group A's series comment to still hold its season")
		require.Equal(t, textGroupASeason1, (*commentGroupADb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group A's season 1 comment to be untouched by the rejected cross-group requests")

		commentGroupBDb := getCommentFromDB(t, commentGroupB.Id)
		require.NotNil(t, commentGroupBDb.SeasonsComments, "Expected group B's series comment to still hold its season")
		require.Equal(t, textGroupBSeason1, (*commentGroupBDb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group B's season 1 comment to be untouched by requests addressed to group A")

		require.Len(t, getCommentsFromDB(t, world.tvSeries.ID), 2,
			"Expected both groups' series comments to survive the rejected cross-group requests")
	})

	t.Run("Commenting on a season already commented in the same group returns 409 and leaves the other group untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		textGroupASeason1 := "group A season 1"
		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.tvSeries.ID, textGroupASeason1, &season1, world.token)
		_ = addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, "group B season 1", &season1, world.token)

		respDuplicate := addComment(t, comments.NewComment{
			GroupId: world.groupB.Id,
			TitleId: world.tvSeries.ID,
			Comment: "group B season 1 again",
			Season:  &season1,
		}, world.token)
		defer respDuplicate.Body.Close()
		require.Equal(t, http.StatusConflict, respDuplicate.StatusCode,
			"Expected commenting on the same season twice in the same group to still conflict")

		var respDuplicateBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDuplicate.Body).Decode(&respDuplicateBody),
			"error decoding the season conflict response body")
		require.Contains(t, respDuplicateBody.ErrorMessage, comments.ErrSeasonCommentAlreadyExists.Error()[1:],
			"Expected the same-group season duplicate to report the season-comment-already-exists error")

		commentGroupADb := getCommentFromDB(t, commentGroupA.Id)
		require.NotNil(t, commentGroupADb.SeasonsComments, "Expected group A's series comment to still hold its season")
		require.Equal(t, textGroupASeason1, (*commentGroupADb.SeasonsComments)[strconv.Itoa(season1)].Comment,
			"Expected group A's season comment to be untouched by a rejected group B insert")
	})

	t.Run("Deleting a season comment leaves the other group's series comment intact", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		commentGroupA := addCommentAndGetResult(t, world.groupA.Id, world.tvSeries.ID, "group A season 1", &season1, world.token)
		commentGroupB := addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, "group B season 1", &season1, world.token)
		_ = addCommentAndGetResult(t, world.groupB.Id, world.tvSeries.ID, "group B season 2", &season2, world.token)

		// Season 1 is group A's only season, so deleting it removes the whole
		// group A comment and must not reach group B's season 1.
		respDeleted := deleteCommentSeasonFromApi(t, world.groupA.Id, world.tvSeries.ID, commentGroupA.Id, world.token, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode, "Expected group A's season comment to be deleted")

		commentsDb := getCommentsFromDB(t, world.tvSeries.ID)
		require.Len(t, commentsDb, 1, "Expected group A's series comment to be removed once its last season was deleted")
		require.Equal(t, commentGroupB.Id, commentsDb[0].Id, "Expected the surviving series comment to be group B's")
		require.NotNil(t, commentsDb[0].SeasonsComments, "Expected group B's series comment to still hold season comments")
		require.Len(t, *commentsDb[0].SeasonsComments, 2,
			"Expected group B to keep both of its seasons after group A's season was deleted")

		require.Empty(t, getCommentsForGroupTitle(t, world.groupA.Id, world.tvSeries.ID, world.token),
			"Expected group A to return no series comment after deleting its last season")

		commentsGroupB := getCommentsForGroupTitle(t, world.groupB.Id, world.tvSeries.ID, world.token)
		require.Len(t, commentsGroupB, 1, "Expected group B to still return its own series comment")
		require.Len(t, *commentsGroupB[0].SeasonsComments, 2, "Expected group B to still return both of its seasons")
	})
}
