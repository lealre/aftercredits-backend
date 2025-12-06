package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddRating(t *testing.T) {
	resetDB(t)

	// ============ TEST SETUP ============

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

	// Users not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	t.Run("Adding a rating sucessfully", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusCreated, respNewRating.StatusCode)

		var respNewRatingBody ratings.Rating
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Equal(t, user.Id, respNewRatingBody.UserId)
		require.Equal(t, expectedTitle.ID, respNewRatingBody.TitleId)
		require.Equal(t, expectedNote, respNewRatingBody.Note)
		require.NotEmpty(t, respNewRatingBody.CreatedAt)
		require.Equal(t, respNewRatingBody.CreatedAt, respNewRatingBody.UpdatedAt)

		// Database assertion
		ratingDb := getRating(t, respNewRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedTitle.ID, ratingDb.TitleId)
		require.Equal(t, expectedNote, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.Equal(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
	})

	t.Run("Adding a rating twice should return 409", func(t *testing.T) {
		expectedNote := float32(8)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusConflict, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrRatingAlreadyExists.Error()[1:])
	})

	t.Run("Adding a rating for a title not in group should return 404", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: titleNotIngroup.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newRating.GroupId, newRating.TitleId), respNewRatingBody.ErrorMessage)
	})

	t.Run("Adding a rating not being from group should return 404", func(t *testing.T) {
		expectedNote := float32(5)
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTitle.ID,
			Note:    expectedNote,
		}

		respNewRating := addRating(t, newRating, tokenUserNotInGroup)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, fmt.Sprintf("Group %s do not have title %s or do not exist.", newRating.GroupId, newRating.TitleId), respNewRatingBody.ErrorMessage)
	})

}
