package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAddRating(t *testing.T) {
	resetDB(t)

	// =========================================================
	// 		TEST SETUP - ADDING RATINGS
	// =========================================================

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

	// =========================================================
	// 		TEST ADDING RATINGS - MOVIES
	// =========================================================

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

	// =========================================================
	// 		TEST ADDING RATINGS - TV SERIES
	// =========================================================

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
		seasonRating := (*respNewRatingBody.SeasonsRatings)[strconv.Itoa(expectedSeason)]
		require.Equal(t, expectedNoteSeasonOne, seasonRating.Rating)
		require.NotEmpty(t, seasonRating.AddedAt)
		require.NotEmpty(t, seasonRating.UpdatedAt)
		require.Equal(t, seasonRating.AddedAt, seasonRating.UpdatedAt)

		// Database assertion
		ratingDb := getRating(t, respNewRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedTVSeriesTitle.ID, ratingDb.TitleId)
		require.Equal(t, expectedNoteSeasonOne, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.Equal(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
		require.NotEmpty(t, ratingDb.SeasonsRatings)
		seasonRatingItem := (*ratingDb.SeasonsRatings)[strconv.Itoa(expectedSeason)]
		require.Equal(t, expectedNoteSeasonOne, seasonRatingItem.Rating)
		require.NotEmpty(t, seasonRatingItem.AddedAt)
		require.NotEmpty(t, seasonRatingItem.UpdatedAt)
		require.Equal(t, seasonRatingItem.AddedAt, seasonRatingItem.UpdatedAt)
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
				seasonRating := (*respNewRatingBody.SeasonsRatings)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedNote, seasonRating.Rating)
				require.NotEmpty(t, seasonRating.AddedAt)
				require.NotEmpty(t, seasonRating.UpdatedAt)

				// Database assertion
				ratingDb := getRating(t, respNewRatingBody.Id)
				require.Equal(t, user.Id, ratingDb.UserId)
				require.Equal(t, expectedTVSeriesTitle.ID, ratingDb.TitleId)
				require.Equal(t, tt.expectedOverall, ratingDb.Note)
				require.NotEmpty(t, ratingDb.CreatedAt)
				require.NotEqual(t, ratingDb.CreatedAt, ratingDb.UpdatedAt) // UpdatedAt should be different from CreatedAt now
				require.True(t, ratingDb.UpdatedAt.After(ratingDb.CreatedAt))
				require.NotEmpty(t, ratingDb.SeasonsRatings)
				seasonRatingItem := (*ratingDb.SeasonsRatings)[strconv.Itoa(tt.season)]
				require.Equal(t, tt.expectedNote, seasonRatingItem.Rating)
				require.NotEmpty(t, seasonRatingItem.AddedAt)
				require.NotEmpty(t, seasonRatingItem.UpdatedAt)
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

	// =========================================================
	// 		TEST SETUP - UPDATING RATINGS
	// =========================================================

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
	expectedMovieTitle := movieTitles[0]
	expectedTVSeriesTitle := tvSeriesTitles[0]

	// Add expected title to update to group
	for _, title := range allTitles {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

	// User not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Add a rating for the movie
	ratingToUpdateMovie := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float32(5), nil, tokenOwnerUser)

	// Add ratings for the TV series (season 1 and season 2)
	season1 := 1
	season2 := 2
	season1Note := float32(5)
	season2Note := float32(8)
	ratingToUpdateTVSeriesSeason1 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, season1Note, &season1, tokenOwnerUser)
	ratingToUpdateTVSeriesSeason2 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, season2Note, &season2, tokenOwnerUser)
	// These variables are available for future TV series update tests
	_ = ratingToUpdateTVSeriesSeason1
	_ = ratingToUpdateTVSeriesSeason2

	// Add delay to ensure UpdatedAt will be different from CreatedAt
	time.Sleep(1 * time.Second)

	// =========================================================
	// 		TEST UPDATE RATINGS - MOVIES
	// =========================================================

	t.Run("Update a movie rating sucessfully", func(t *testing.T) {
		expectedNewNote := float32(10)
		updateRequestRating := ratings.UpdateRatingRequest{
			Note: expectedNewNote,
		}

		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateMovie.Id, tokenOwnerUser)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusOK, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody ratings.Rating
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Equal(t, user.Id, respUpdatedRatingBody.UserId)
		require.Equal(t, expectedMovieTitle.ID, respUpdatedRatingBody.TitleId)
		require.Equal(t, expectedNewNote, respUpdatedRatingBody.Note)
		require.NotEmpty(t, respUpdatedRatingBody.CreatedAt)
		require.NotEqual(t, respUpdatedRatingBody.CreatedAt, respUpdatedRatingBody.UpdatedAt)
		require.True(t, respUpdatedRatingBody.UpdatedAt.After(respUpdatedRatingBody.CreatedAt))

		// Database assertion
		ratingDb := getRating(t, respUpdatedRatingBody.Id)
		require.Equal(t, user.Id, ratingDb.UserId)
		require.Equal(t, expectedMovieTitle.ID, ratingDb.TitleId)
		require.Equal(t, expectedNewNote, ratingDb.Note)
		require.NotEmpty(t, ratingDb.CreatedAt)
		require.NotEqual(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
		require.True(t, ratingDb.UpdatedAt.After(ratingDb.CreatedAt))
	})

	t.Run("Update a movie rating from other user should return 404", func(t *testing.T) {
		expectedNewNote := float32(10)
		updateRequestRating := ratings.UpdateRatingRequest{
			Note: expectedNewNote,
		}

		// This user is not the owner of the rating. Here we are testing only the rating permissions, unrelated to the group.
		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateMovie.Id, tokenUserNotInGroup)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrRatingNotFound.Error()[1:])
	})

	t.Run("Update a movie rating with notes not between 0 and 10 should return 400", func(t *testing.T) {
		expectedNotes := []float32{-5, 11}

		for _, note := range expectedNotes {
			updateRequestRating := ratings.UpdateRatingRequest{
				Note: note,
			}

			respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateMovie.Id, tokenOwnerUser)
			defer respUpdatedRating.Body.Close()
			require.Equal(t, http.StatusBadRequest, respUpdatedRating.StatusCode)

			var respUpdatedRatingBody api.ErrorResponse
			require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
			require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrInvalidNoteValue.Error()[1:])
		}
	})

	// =========================================================
	// 		TEST UPDATE RATINGS - TV SERIES
	// =========================================================

	seasonTests := []struct {
		name         string
		season       int
		ratingId     string
		newNote      float32
		expectedNote func(seasonsRatings *ratings.SeasonsRatings) float32
	}{
		{
			name:     "season 1",
			season:   season1,
			ratingId: ratingToUpdateTVSeriesSeason1.Id,
			newNote:  float32(10),
			expectedNote: func(seasonsRatings *ratings.SeasonsRatings) float32 {
				return (float32(10) + season2Note) / 2
			},
		},
		{
			name:     "season 2",
			season:   season2,
			ratingId: ratingToUpdateTVSeriesSeason2.Id,
			newNote:  float32(3),
			expectedNote: func(seasonsRatings *ratings.SeasonsRatings) float32 {
				var sum float32
				for _, seasonRating := range *seasonsRatings {
					sum += seasonRating.Rating
				}
				return sum / float32(len(*seasonsRatings))
			},
		},
	}

	for _, tt := range seasonTests {
		t.Run(fmt.Sprintf("Update a TV series rating %s sucessfully", tt.name), func(t *testing.T) {
			updateRequestRating := ratings.UpdateRatingRequest{
				Note:   tt.newNote,
				Season: &tt.season,
			}

			respUpdatedRating := updateRating(t, updateRequestRating, tt.ratingId, tokenOwnerUser)
			defer respUpdatedRating.Body.Close()
			require.Equal(t, http.StatusOK, respUpdatedRating.StatusCode)

			var respUpdatedRatingBody ratings.Rating
			require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
			require.Equal(t, user.Id, respUpdatedRatingBody.UserId)
			require.Equal(t, expectedTVSeriesTitle.ID, respUpdatedRatingBody.TitleId)
			seasonRating := (*respUpdatedRatingBody.SeasonsRatings)[strconv.Itoa(tt.season)]
			require.Equal(t, tt.newNote, seasonRating.Rating)
			require.Equal(t, tt.expectedNote(respUpdatedRatingBody.SeasonsRatings), respUpdatedRatingBody.Note)
			require.NotEmpty(t, respUpdatedRatingBody.CreatedAt)
			require.NotEqual(t, respUpdatedRatingBody.CreatedAt, respUpdatedRatingBody.UpdatedAt)
			require.True(t, respUpdatedRatingBody.UpdatedAt.After(respUpdatedRatingBody.CreatedAt))
			require.NotEmpty(t, respUpdatedRatingBody.SeasonsRatings)
			require.NotEmpty(t, seasonRating.AddedAt)
			require.NotEmpty(t, seasonRating.UpdatedAt)
			require.True(t, seasonRating.UpdatedAt.After(seasonRating.AddedAt) || seasonRating.UpdatedAt.Equal(seasonRating.AddedAt))

			// Database assertion
			ratingDb := getRating(t, respUpdatedRatingBody.Id)
			require.Equal(t, user.Id, ratingDb.UserId)
			require.Equal(t, expectedTVSeriesTitle.ID, ratingDb.TitleId)
			seasonRatingItem := (*ratingDb.SeasonsRatings)[strconv.Itoa(tt.season)]
			require.Equal(t, tt.newNote, seasonRatingItem.Rating)
			require.Equal(t, tt.expectedNote(respUpdatedRatingBody.SeasonsRatings), ratingDb.Note)
			require.NotEmpty(t, ratingDb.CreatedAt)
			require.NotEqual(t, ratingDb.CreatedAt, ratingDb.UpdatedAt)
			require.True(t, ratingDb.UpdatedAt.After(ratingDb.CreatedAt))
			require.NotEmpty(t, ratingDb.SeasonsRatings)
			require.NotEmpty(t, seasonRatingItem.AddedAt)
			require.NotEmpty(t, seasonRatingItem.UpdatedAt)
			require.True(t, seasonRatingItem.UpdatedAt.After(seasonRatingItem.AddedAt) || seasonRatingItem.UpdatedAt.Equal(seasonRatingItem.AddedAt))
		})
	}

	t.Run("Update a TV series rating with invalid season value should return 400", func(t *testing.T) {
		invalidSeasons := []int{0, -1}

		for _, season := range invalidSeasons {
			invalidSeason := season
			updateRequestRating := ratings.UpdateRatingRequest{
				Note:   float32(10),
				Season: &invalidSeason,
			}

			respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateTVSeriesSeason1.Id, tokenOwnerUser)
			defer respUpdatedRating.Body.Close()
			require.Equal(t, http.StatusBadRequest, respUpdatedRating.StatusCode)

			var respUpdatedRatingBody api.ErrorResponse
			require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
			require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrInvalidSeasonValue.Error()[1:])
		}
	})

	t.Run("Update a TV series rating with season that has no rating should return 404", func(t *testing.T) {
		seasonWithoutRating := 3
		updateRequestRating := ratings.UpdateRatingRequest{
			Note:   float32(10),
			Season: &seasonWithoutRating,
		}

		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateTVSeriesSeason1.Id, tokenOwnerUser)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusNotFound, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrRatingNotFound.Error()[1:])
	})

	t.Run("Update a TV series rating without season value in request should return 400", func(t *testing.T) {
		updateRequestRating := ratings.UpdateRatingRequest{
			Note: float32(10),
		}

		respUpdatedRating := updateRating(t, updateRequestRating, ratingToUpdateTVSeriesSeason1.Id, tokenOwnerUser)
		defer respUpdatedRating.Body.Close()
		require.Equal(t, http.StatusBadRequest, respUpdatedRating.StatusCode)

		var respUpdatedRatingBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respUpdatedRating.Body).Decode(&respUpdatedRatingBody))
		require.Contains(t, respUpdatedRatingBody.ErrorMessage, ratings.ErrSeasonRequired.Error()[1:])
	})
}

func TestDeleteRating(t *testing.T) {
	resetDB(t)

	// =========================================================
	// 		TEST SETUP - DELETING RATINGS
	// =========================================================

	// Create a new user
	_, tokenOwnerUser := addUser(t, users.NewUserRequest{
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

	expectedMovieTitle := movieTitles[0]
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

	// =========================================================
	// 		TEST DELETING RATINGS - MOVIES
	// =========================================================

	t.Run("Deleting a movie rating successfully", func(t *testing.T) {
		// Add a rating for the movie
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float32(5), nil, tokenOwnerUser)

		// Delete the rating
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Rating with id %s deleted successfully", ratingToDelete.Id), respBody.Message)

		// DB assertion: rating should not exist
		ctx := context.Background()
		db := testClient.Database(TEST_DB_NAME)
		coll := db.Collection(mongodb.RatingsCollection)
		var ratingDb mongodb.RatingDb
		err := coll.FindOne(ctx, bson.M{"_id": ratingToDelete.Id}).Decode(&ratingDb)
		require.Error(t, err, "Expected rating to be deleted from database")
	})

	t.Run("Deleting a rating that does not exist should return 404", func(t *testing.T) {
		nonExistentRatingId := "507f1f77bcf86cd799439011"
		respDeleted := deleteRating(t, nonExistentRatingId, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, fmt.Sprintf("Rating with id %s not found", nonExistentRatingId))
	})

	t.Run("Deleting a rating that belongs to another user should return 404", func(t *testing.T) {
		// Add a rating for the movie by the owner
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float32(5), nil, tokenOwnerUser)

		// Try to delete it with another user's token
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenUserNotInGroup)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, fmt.Sprintf("Rating with id %s not found", ratingToDelete.Id))
	})

	// =========================================================
	// 		TEST DELETING RATINGS - TV SERIES (ENTIRE RATING)
	// =========================================================

	t.Run("Deleting a TV series rating successfully (entire rating)", func(t *testing.T) {
		// Add ratings for TV series (season 1 and season 2)
		season1 := 1
		season2 := 2
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float32(5), &season1, tokenOwnerUser)
		_ = addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float32(8), &season2, tokenOwnerUser)

		// Delete the entire rating
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Rating with id %s deleted successfully", ratingToDelete.Id), respBody.Message)

		// DB assertion: rating should not exist
		ctx := context.Background()
		db := testClient.Database(TEST_DB_NAME)
		coll := db.Collection(mongodb.RatingsCollection)
		var ratingDb mongodb.RatingDb
		err := coll.FindOne(ctx, bson.M{"_id": ratingToDelete.Id}).Decode(&ratingDb)
		require.Error(t, err, "Expected rating to be deleted from database")
	})
}

func TestDeleteRatingSeason(t *testing.T) {
	resetDB(t)

	// =========================================================
	// 		TEST SETUP - DELETING SEASON RATINGS
	// =========================================================

	// Create a new user
	_, tokenOwnerUser := addUser(t, users.NewUserRequest{
		Username: "testname",
		Password: "testpass",
	})

	// Create a group for user
	group := createGroup(t, groups.CreateGroupRequest{
		Name: "testgroupname",
	}, tokenOwnerUser)

	// Add titles to database
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	seedTitles(t, tvSeriesTitles)

	expectedTVSeriesTitle := tvSeriesTitles[0]
	expectedTVSeriesTitle2 := tvSeriesTitles[1]

	// Add tv series titles to group
	for _, title := range []imdb.Title{expectedTVSeriesTitle, expectedTVSeriesTitle2} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
			GroupId: group.Id,
		}, tokenOwnerUser)
	}

	// User not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// Create a TV series rating with seasons 1 and 2
	season1 := 1
	season2 := 2
	ratingSeason1 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float32(5), &season1, tokenOwnerUser)
	ratingSeason2 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float32(8), &season2, tokenOwnerUser)
	require.Equal(t, ratingSeason1.Id, ratingSeason2.Id, "Expected same rating id for multiple seasons of the same TV series")

	ratingId := ratingSeason1.Id

	// ======================================================================
	// 		TEST DELETING SEASON RATINGS
	// ======================================================================

	t.Run("Deleting a season rating successfully", func(t *testing.T) {
		respDeleted := deleteRatingSeason(t, ratingId, tokenOwnerUser, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Season %d from rating %s deleted successfully", season1, ratingId), respBody.Message)

		// DB assertion: rating still exists, but season 1 was removed
		ratingDb := getRating(t, ratingId)
		require.NotNil(t, ratingDb.SeasonsRatings)
		_, ok := (*ratingDb.SeasonsRatings)["1"]
		require.False(t, ok, "Expected season 1 to be deleted from SeasonsRatings")
		season2RatingDb := (*ratingDb.SeasonsRatings)["2"]
		require.Equal(t, float32(8), season2RatingDb.Rating)
		require.NotEmpty(t, season2RatingDb.AddedAt)
		require.NotEmpty(t, season2RatingDb.UpdatedAt)
		// Overall rating should be recalculated (only season 2 remains)
		require.Equal(t, float32(8), ratingDb.Note)
	})

	t.Run("Deleting last season should delete the whole rating document", func(t *testing.T) {
		onlySeason := 1
		ratingOnlySeason := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle2.ID, float32(7), &onlySeason, tokenOwnerUser)

		respDeleted := deleteRatingSeason(t, ratingOnlySeason.Id, tokenOwnerUser, onlySeason)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Season %d from rating %s deleted successfully", onlySeason, ratingOnlySeason.Id), respBody.Message)

		// DB assertion: rating should not exist
		ctx := context.Background()
		db := testClient.Database(TEST_DB_NAME)
		coll := db.Collection(mongodb.RatingsCollection)
		var ratingDb mongodb.RatingDb
		err := coll.FindOne(ctx, bson.M{"_id": ratingOnlySeason.Id}).Decode(&ratingDb)
		require.Error(t, err, "Expected rating to be deleted from database")
	})

	t.Run("Deleting a season rating with invalid season should return 400", func(t *testing.T) {
		respDeleted := deleteRatingSeason(t, ratingId, tokenOwnerUser, 0)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusBadRequest, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, ratings.ErrInvalidSeasonValue.Error()[1:])
	})

	t.Run("Deleting a season rating that does not exist should return 404", func(t *testing.T) {
		nonExistentSeason := 100
		respDeleted := deleteRatingSeason(t, ratingId, tokenOwnerUser, nonExistentSeason)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusBadRequest, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, ratings.ErrSeasonDoesNotExist.Error()[1:])
	})

	t.Run("Deleting a season rating from a rating that does not exist should return 404", func(t *testing.T) {
		nonExistentRatingId := "507f1f77bcf86cd799439011"
		respDeleted := deleteRatingSeason(t, nonExistentRatingId, tokenOwnerUser, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, fmt.Sprintf("Rating with id %s not found", nonExistentRatingId))
	})

	t.Run("Deleting a season rating that belongs to another user should return 404", func(t *testing.T) {
		// Add a rating for TV series by the owner
		season1 := 1
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float32(5), &season1, tokenOwnerUser)

		// Try to delete it with another user's token
		respDeleted := deleteRatingSeason(t, ratingToDelete.Id, tokenUserNotInGroup, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, fmt.Sprintf("Rating with id %s not found", ratingToDelete.Id))
	})
}
