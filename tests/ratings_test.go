package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddRating(t *testing.T) {
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
	movieTitles := loadTitlesFixture(t)
	seedTitles(t, movieTitles)
	expectedMovieTitle := movieTitles[0]
	movieTitleNotIngroup := movieTitles[2]

	// Add expected title to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedMovieTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// User not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// ===================================
	// 		TEST ADDING RATINGS - MOVIES
	// ===================================

	t.Run("Adding a rating for a movie title sucessfully", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusCreated, respNewRating.StatusCode)

		var respNewRatingBody ratings.Rating
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Equal(t, user.Id, respNewRatingBody.UserId)
		require.Equal(t, expectedMovieTitle.ID, respNewRatingBody.TitleId)
		require.Equal(t, expectedNote, respNewRatingBody.Note)
		require.NotEmpty(t, respNewRatingBody.CreatedAt)
		require.Equal(t, respNewRatingBody.CreatedAt, respNewRatingBody.UpdatedAt)

		// Database assertion
		ratingDb := getRating(t, respNewRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedMovieTitle.ID, ratingDb.TitleId)
		require.Equal(t, expectedNote, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.Equal(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
	})

	t.Run("Adding a rating for a movie title twice should return 409", func(t *testing.T) {
		expectedNote := float32(8)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusConflict, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrRatingAlreadyExists.Error()[1:])
	})

	t.Run("Adding a rating for a movie title not in group should return 404", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: movieTitleNotIngroup.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newRating.GroupId, newRating.TitleId), respNewRatingBody.ErrorMessage)
	})

	t.Run("Adding a rating for a movie title not being from group should return 404", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedMovieTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenUserNotInGroup)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newRating.GroupId, newRating.TitleId), respNewRatingBody.ErrorMessage)
	})

	t.Run("Add a rating for a movie title with notes not between 0 and 10 should return 400", func(t *testing.T) {
		expectedNotes := []float32{-5, 11}

		for _, note := range expectedNotes {
			newRating := ratings.NewRating{
				GroupId: group.Id,
				TitleId: expectedMovieTitle.ID,
				Note:    note,
			}

			respUpdatedRating := addRating(t, newRating, tokenOwnerUser)
			defer respUpdatedRating.Body.Close()
			require.Equal(t, http.StatusBadRequest, respUpdatedRating.StatusCode)

			var respUpdatedRatingBody api.ErrorResponse
			require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
			require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrInvalidNoteValue.Error()[1:])
		}
	})
}

func TestUpdateRating(t *testing.T) {
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
	expectedTitleToUpdate := titles[1]

	// Add expected title to update to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitleToUpdate.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// User not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Add a rating to be updated
	expectedNote := float32(5)
	newRating := ratings.NewRating{
		GroupId: group.Id,
		TitleId: expectedTitleToUpdate.ID,
		Note:    expectedNote,
	}

	respNewRating := addRating(t, newRating, tokenOwnerUser)
	defer respNewRating.Body.Close()
	require.Equal(t, http.StatusCreated, respNewRating.StatusCode)
	var ratingToUpdate ratings.Rating
	require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&ratingToUpdate))

	// Add delay to ensure UpdatedAt will be different from CreatedAt
	time.Sleep(1 * time.Second)

	// ===================================
	// 		TEST UPDATE RATINGS
	// ===================================

	t.Run("Update a rating sucessfully", func(t *testing.T) {
		expectedNewNote := float32(10)
		updateRequestRating := ratings.UpdateRatingRequest{
			Note: expectedNewNote,
		}

		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdate.Id, tokenOwnerUser)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusOK, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody ratings.Rating
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Equal(t, user.Id, respUpdatedRatingBody.UserId)
		require.Equal(t, expectedTitleToUpdate.ID, respUpdatedRatingBody.TitleId)
		require.Equal(t, expectedNewNote, respUpdatedRatingBody.Note)
		require.NotEmpty(t, respUpdatedRatingBody.CreatedAt)
		require.NotEqual(t, respUpdatedRatingBody.CreatedAt, respUpdatedRatingBody.UpdatedAt)
		require.True(t, respUpdatedRatingBody.UpdatedAt.After(respUpdatedRatingBody.CreatedAt))

		// Database assertion
		ratingDb := getRating(t, respUpdatedRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedTitleToUpdate.ID, ratingDb.TitleId)
		require.Equal(t, expectedNewNote, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.NotEqual(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
		require.True(t, ratingDb.UpdatedAt.After(ratingDb.CreatedAt))
	})

	t.Run("Update a rating from other user should return 404", func(t *testing.T) {
		expectedNewNote := float32(10)
		updateRequestRating := ratings.UpdateRatingRequest{
			Note: expectedNewNote,
		}

		// This user is not the owner of the rating. Here we are testing only the rating permissions, unrelated to the group.
		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdate.Id, tokenUserNotInGroup)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrRatingNotFound.Error()[1:])
	})

	t.Run("Update a rating with notes not between 0 and 10 should return 400", func(t *testing.T) {
		expectedNotes := []float32{-5, 11}

		for _, note := range expectedNotes {
			updateRequestRating := ratings.UpdateRatingRequest{
				Note: note,
			}

			respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdate.Id, tokenOwnerUser)
			defer respUpdatedRating.Body.Close()
			require.Equal(t, http.StatusBadRequest, respUpdatedRating.StatusCode)

			var respUpdatedRatingBody api.ErrorResponse
			require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
			require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrInvalidNoteValue.Error()[1:])
		}
	})
}
