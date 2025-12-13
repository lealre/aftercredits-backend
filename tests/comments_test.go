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

	// Expected comment to be used in tests
	expectedComment := "This is a test comment"

	// ======================================================================
	// 		TEST ADDING COMMENTS
	// ======================================================================

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
		commentDb := getCommentFromDB(t, respNewCommentBody.Id)
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
		require.Equal(t, expectedComment, respGetCommentsBody.Comments[0].Comment)
		require.NotEmpty(t, respGetCommentsBody.Comments[0].CreatedAt)
		require.Equal(t, respGetCommentsBody.Comments[0].CreatedAt, respGetCommentsBody.Comments[0].UpdatedAt)

		// Database assertion
		commentDb := getCommentsFromDB(t, expectedTitle.ID)
		require.Equal(t, 1, len(commentDb))
		require.Equal(t, user.Id, commentDb[0].UserId)
		require.Equal(t, expectedTitle.ID, commentDb[0].TitleId)
		require.Equal(t, expectedComment, commentDb[0].Comment)
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
	comment := addComment(t, comments.NewComment{
		GroupId: group.Id,
		TitleId: expectedTitle.ID,
		Comment: "This is a test comment",
	}, tokenOwnerUser)
	defer comment.Body.Close()
	require.Equal(t, http.StatusCreated, comment.StatusCode)
	var commentCreated comments.Comment
	require.NoError(t, json.NewDecoder(comment.Body).Decode(&commentCreated))

	// ======================================================================
	// 		TEST UPDATING COMMENTS
	// ======================================================================

	t.Run("Updating a comment sucessfully", func(t *testing.T) {
		expectedUpdatedComment := "This is a test comment updated"
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTitle.ID, commentCreated.Id, expectedUpdatedComment, tokenOwnerUser)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusOK, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody comments.Comment
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Equal(t, user.Id, respUpdatedCommentBody.UserId)
		require.Equal(t, expectedTitle.ID, respUpdatedCommentBody.TitleId)
		require.Equal(t, expectedUpdatedComment, respUpdatedCommentBody.Comment)
		require.NotEmpty(t, respUpdatedCommentBody.CreatedAt)
		require.NotEqual(t, respUpdatedCommentBody.CreatedAt, respUpdatedCommentBody.UpdatedAt)
		require.True(t, respUpdatedCommentBody.UpdatedAt.After(respUpdatedCommentBody.CreatedAt))

		// Database assertion
		commentDb := getCommentFromDB(t, respUpdatedCommentBody.Id)
		require.Equal(t, user.Id, commentDb.UserId)
		require.Equal(t, expectedTitle.ID, commentDb.TitleId)
		require.Equal(t, expectedUpdatedComment, commentDb.Comment)
		require.NotEmpty(t, commentDb.CreatedAt)
		require.NotEqual(t, commentDb.CreatedAt, commentDb.UpdatedAt)
		require.True(t, commentDb.UpdatedAt.After(commentDb.CreatedAt))
	})

	t.Run("Updating a comment that is not from the user should return 404", func(t *testing.T) {
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTitle.ID, commentCreated.Id, "This is a test comment updated", tokenUserNotInGroup)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, fmt.Sprintf("Group %s do not have title %s or do not exist.", group.Id, expectedTitle.ID))
	})

	t.Run("Updating a comment with a empty comment should return 400", func(t *testing.T) {
		respUpdatedComment := updateCommentFromApi(t, group.Id, expectedTitle.ID, commentCreated.Id, "   ", tokenOwnerUser)
		defer respUpdatedComment.Body.Close()
		require.Equal(t, http.StatusBadRequest, respUpdatedComment.StatusCode)

		var respUpdatedCommentBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedComment.Body).Decode(&respUpdatedCommentBody))
		require.Contains(t, respUpdatedCommentBody.ErrorMessage, comments.ErrCommentIsNull.Error()[1:])
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
		require.Equal(t, expectedGroupComment, commentDb[0].Comment)
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
		require.Equal(t, expectedGroupComment, commentDb[0].Comment)
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
		require.Equal(t, expectedGroupComment, commentDb[0].Comment)
		require.NotEmpty(t, commentDb[0].CreatedAt)
		require.Equal(t, commentDb[0].CreatedAt, commentDb[0].UpdatedAt)
	})
}
