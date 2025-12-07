package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddComment(t *testing.T) {
	resetDB(t)

	// ===================================
	// 		TEST SETUP
	// ===================================

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
	expectedTitleToUpdate := titles[1]
	titleNotIngroup := titles[2]

	// Add expected title to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// Add expected title to update to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitleToUpdate.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// Users not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Expected comment to be used in tests
	expectedComment := "This is a test comment"

	// ===================================
	// 		TEST ADDING COMMENTS
	// ===================================

	t.Run("Adding a comment sucessfully", func(t *testing.T) {

		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusCreated, respNewComment.StatusCode)

		var respNewCommentBody comments.Comment
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Equal(t, user.Id, respNewCommentBody.UserId)
		require.Equal(t, expectedTitle.ID, respNewCommentBody.TitleId)
		require.Equal(t, expectedComment, respNewCommentBody.Comment)
		require.NotEmpty(t, respNewCommentBody.CreatedAt)
		require.Equal(t, respNewCommentBody.CreatedAt, respNewCommentBody.UpdatedAt)

		// Database assertion
		commentDb := getComment(t, respNewCommentBody.Id)
		require.Equal(t, user.Id, commentDb.UserId)
		require.Equal(t, expectedTitle.ID, commentDb.TitleId)
		require.Equal(t, expectedComment, commentDb.Comment)
		require.NotEmpty(t, commentDb.CreatedAt)
		require.Equal(t, commentDb.CreatedAt, commentDb.UpdatedAt)
	})

	t.Run("Adding a comment with a empty comment should return 400", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Comment: "   ",
		}
		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrCommentIsNull.Error()[1:])
	})

	t.Run("Adding a comment from the same user and title should return 409", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusConflict, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, respNewCommentBody.ErrorMessage, comments.ErrCommentAlreadyExists.Error()[1:])
	})

	t.Run("Adding a comment for a title that is not in the group should return 404", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: titleNotIngroup.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenOwnerUser)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId), respNewCommentBody.ErrorMessage)
	})

	t.Run("Adding a comment from user that is not in the group should return 404", func(t *testing.T) {
		newComment := comments.NewComment{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Comment: expectedComment,
		}

		respNewComment := addComment(t, newComment, tokenUserNotInGroup)
		defer respNewComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewComment.StatusCode)

		var respNewCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewComment.Body).Decode(&respNewCommentBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId), respNewCommentBody.ErrorMessage)
	})
}
