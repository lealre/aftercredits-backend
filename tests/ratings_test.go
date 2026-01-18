package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	allTitles := append(movieTitles, tvSeriesTitles...)
	seedTitles(t, allTitles)

	// Get expected titles to tests assertions
	expectedMovieTitle := movieTitles[0]
	expectedMovieTitleNotIngroup := movieTitles[2]
	expectedTVSeriesTitle := tvSeriesTitles[0]

	// Add titles to group
	for _, title := range []string{expectedMovieTitle.ID, expectedTVSeriesTitle.ID} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

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
			TitleId: expectedMovieTitleNotIngroup.ID,
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

	// ===================================
	// 		TEST ADDING RATINGS - TV SERIES
	// ===================================

	expectedNoteSeasonOne := float32(5)
	expectedNoteSeasonTwo := float32(8)
	expectedNoteSeasonThree := float32(10)

	t.Run("Adding a rating for a TV series for the first time should create a new rating sucessfully", func(t *testing.T) {
		expectedSeason := 1
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Note:    expectedNoteSeasonOne,
			Season:  &expectedSeason,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusCreated, respNewRating.StatusCode)

		var respNewRatingBody ratings.Rating
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Equal(t, user.Id, respNewRatingBody.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, respNewRatingBody.TitleId)
		require.Equal(t, expectedNoteSeasonOne, respNewRatingBody.Note)
		require.NotEmpty(t, respNewRatingBody.CreatedAt)
		require.Equal(t, respNewRatingBody.CreatedAt, respNewRatingBody.UpdatedAt)
		require.NotEmpty(t, respNewRatingBody.SeasonsRatings)
		require.Equal(t, expectedNoteSeasonOne, (*respNewRatingBody.SeasonsRatings)[strconv.Itoa(expectedSeason)])

		// Database assertion
		ratingDb := getRating(t, respNewRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, ratingDb.TitleId)
		require.Equal(t, expectedNoteSeasonOne, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.Equal(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
		require.NotEmpty(t, ratingDb.SeasonsRatings)
		require.Equal(t, expectedNoteSeasonOne, (*ratingDb.SeasonsRatings)[strconv.Itoa(expectedSeason)])
	})

	t.Run("Adding a rating for a TV series for other season should update the rating sucessfully", func(t *testing.T) {
		// In this test we are adding ratings for seasons that do not have a rating yet, but the rating for the first season already exists.
		// We expect to see the overall rating updated to the mean of all seasons ratings, and the additional season rating added.
		seasonTests := []struct {
			season          int
			expectedNote    float32
			expectedOverall float32
		}{
			{season: 2, expectedNote: expectedNoteSeasonTwo, expectedOverall: (expectedNoteSeasonOne + expectedNoteSeasonTwo) / 2},
			{season: 3, expectedNote: expectedNoteSeasonThree, expectedOverall: (expectedNoteSeasonOne + expectedNoteSeasonTwo + expectedNoteSeasonThree) / 3},
		}

		for _, tt := range seasonTests {
			t.Run(fmt.Sprintf("Season %d", tt.season), func(t *testing.T) {
				newRating := ratings.NewRating{
					GroupId: group.Id,
					TitleId: expectedTVSeriesTitle.ID,
					Note:    tt.expectedNote,
					Season:  &tt.season,
				}

				respNewRating := addRating(t, newRating, tokenOwnerUser)
				defer respNewRating.Body.Close()
				require.Equal(t, http.StatusCreated, respNewRating.StatusCode)

				var respNewRatingBody ratings.Rating
				require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
				require.Equal(t, user.Id, respNewRatingBody.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, respNewRatingBody.TitleId)
				require.Equal(t, tt.expectedOverall, respNewRatingBody.Note)
				require.NotEmpty(t, respNewRatingBody.CreatedAt)
				require.NotEqual(t, respNewRatingBody.CreatedAt, respNewRatingBody.UpdatedAt) // UpdatedAt should be different from CreatedAt now
				require.True(t, respNewRatingBody.UpdatedAt.After(respNewRatingBody.CreatedAt))
				require.NotEmpty(t, respNewRatingBody.SeasonsRatings)
				require.Equal(t, tt.expectedNote, (*respNewRatingBody.SeasonsRatings)[strconv.Itoa(tt.season)])

				// Database assertion
				ratingDb := getRating(t, respNewRatingBody.Id)
				require.Equal(t, user.Id, ratingDb.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, ratingDb.TitleId)
				require.Equal(t, tt.expectedOverall, ratingDb.Note)
				require.NotEmpty(t, ratingDb.CreatedAt)
				require.NotEqual(t, ratingDb.CreatedAt, ratingDb.UpdatedAt) // UpdatedAt should be different from CreatedAt now
				require.True(t, ratingDb.UpdatedAt.After(ratingDb.CreatedAt))
				require.NotEmpty(t, ratingDb.SeasonsRatings)
				require.Equal(t, tt.expectedNote, (*ratingDb.SeasonsRatings)[strconv.Itoa(tt.season)])
			})
		}
	})

	t.Run("Adding a rating for a TV series for a season that do not exist should return 404", func(t *testing.T) {
		expectedSeason := 100
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Note:    expectedNoteSeasonOne,
			Season:  &expectedSeason,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrSeasonDoesNotExist.Error()[1:])
	})

	t.Run("Adding a rating for a TV series for a season that already has a rating should return 409", func(t *testing.T) {
		expectedSeason := 1
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Note:    expectedNoteSeasonTwo,
			Season:  &expectedSeason,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusConflict, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrSeasonRatingAlreadyExists.Error()[1:])
	})

	t.Run("Adding a rating for a TV series for a season without a season number should return 400", func(t *testing.T) {
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Note:    expectedNoteSeasonTwo,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrSeasonRequired.Error()[1:])
	})

	t.Run("Adding a rating for a TV series for a season with a season number less than 1 should return 400", func(t *testing.T) {
		expectedSeason := 0
		newRating := ratings.NewRating{
			GroupId: group.Id,
			TitleId: expectedTVSeriesTitle.ID,
			Note:    expectedNoteSeasonTwo,
			Season:  &expectedSeason,
		}

		respNewRating := addRating(t, newRating, tokenOwnerUser)
		defer respNewRating.Body.Close()
		require.Equal(t, http.StatusBadRequest, respNewRating.StatusCode)

		var respNewRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respNewRating.Body).Decode(&respNewRatingBody))
		require.Contains(t, respNewRatingBody.ErrorMessage, ratings.ErrInvalidSeasonValue.Error()[1:])
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
