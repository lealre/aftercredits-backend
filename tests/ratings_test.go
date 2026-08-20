package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestGetRatingById(t *testing.T) {
	resetDB(t)

	// =========================================================
	// 		TEST SETUP - GETTING RATINGS BY ID
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
	seedTitles(t, movieTitles)
	expectedMovieTitle := movieTitles[0]

	// Add title to group
	addTitleToGroup(t, groups.AddTitleToGroupRequest{
		URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedMovieTitle.ID),
		GroupId: group.Id,
	}, tokenOwnerUser)

	// Add a rating
	ratingCreated := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float64(7), nil, tokenOwnerUser)

	// User not in group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "othertestname",
		Password: "testpass",
	})

	// =========================================================
	// 		TEST GET RATING BY ID
	// =========================================================

	t.Run("Get rating by id successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/ratings/"+ratingCreated.Id,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenOwnerUser)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var respRating ratings.Rating
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respRating))
		require.Equal(t, ratingCreated.Id, respRating.Id)
		require.Equal(t, user.Id, respRating.UserId)
		require.Equal(t, expectedMovieTitle.ID, respRating.TitleId)
		require.Equal(t, float64(7), respRating.Note)
		require.NotEmpty(t, respRating.CreatedAt)
		require.NotEmpty(t, respRating.UpdatedAt)
	})

	t.Run("Get rating by id that does not exist should return 404", func(t *testing.T) {
		nonExistentRatingId := "507f1f77bcf86cd799439011"
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/ratings/"+nonExistentRatingId,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenOwnerUser)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})

	t.Run("Get rating by id that belongs to another user should return 404", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/ratings/"+ratingCreated.Id,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserNotInGroup)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})
}

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
		expectedNote := float64(5)
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
		expectedNote := float64(8)
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
		expectedNote := float64(5)
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
		expectedNote := float64(5)
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
		expectedNotes := []float64{-5, 11}

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

	expectedNoteSeasonOne := float64(5)
	expectedNoteSeasonTwo := float64(8)
	expectedNoteSeasonThree := float64(10)

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
			expectedNote    float64
			expectedOverall float64
		}{
			// The overall note is the mean of the season ratings, rounded to
			// one decimal like every other note. (5+8)/2 is already 6.5;
			// (5+8+10)/3 is 7.666… and is stored as 7.7, so the expectation
			// is spelled out rather than recomputed as a raw mean.
			{season: 2, expectedNote: expectedNoteSeasonTwo, expectedOverall: 6.5},
			{season: 3, expectedNote: expectedNoteSeasonThree, expectedOverall: 7.7},
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
	ratingToUpdateMovie := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float64(5), nil, tokenOwnerUser)

	// Add ratings for the TV series (season 1 and season 2)
	season1 := 1
	season2 := 2
	season1Note := float64(5)
	season2Note := float64(8)
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
		expectedNewNote := float64(10)
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
		expectedNewNote := float64(10)
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
		expectedNotes := []float64{-5, 11}

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
		newNote      float64
		expectedNote func(seasonsRatings *ratings.SeasonsRatings) float64
	}{
		{
			name:     "season 1",
			season:   season1,
			ratingId: ratingToUpdateTVSeriesSeason1.Id,
			newNote:  float64(10),
			expectedNote: func(seasonsRatings *ratings.SeasonsRatings) float64 {
				return (float64(10) + season2Note) / 2
			},
		},
		{
			name:     "season 2",
			season:   season2,
			ratingId: ratingToUpdateTVSeriesSeason2.Id,
			newNote:  float64(3),
			expectedNote: func(seasonsRatings *ratings.SeasonsRatings) float64 {
				var sum float64
				for _, seasonRating := range *seasonsRatings {
					sum += seasonRating.Rating
				}
				return sum / float64(len(*seasonsRatings))
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
				Note:   float64(10),
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
			Note:   float64(10),
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
			Note: float64(10),
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
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float64(5), nil, tokenOwnerUser)

		// Delete the rating
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Rating with id %s deleted successfully", ratingToDelete.Id), respBody.Message)

		// DB assertion: rating should not exist
		var n int
		require.NoError(t, testPool.QueryRow(context.Background(),
			"SELECT count(*) FROM ratings WHERE id = $1", ratingToDelete.Id).Scan(&n))
		require.Zero(t, n, "Expected rating to be deleted from database")
	})

	t.Run("Deleting a rating that does not exist should return 404", func(t *testing.T) {
		nonExistentRatingId := "507f1f77bcf86cd799439011"
		respDeleted := deleteRating(t, nonExistentRatingId, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})

	t.Run("Deleting a rating that belongs to another user should return 404", func(t *testing.T) {
		// Add a rating for the movie by the owner
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedMovieTitle.ID, float64(5), nil, tokenOwnerUser)

		// Try to delete it with another user's token
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenUserNotInGroup)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})

	// =========================================================
	// 		TEST DELETING RATINGS - TV SERIES (ENTIRE RATING)
	// =========================================================

	t.Run("Deleting a TV series rating successfully (entire rating)", func(t *testing.T) {
		// Add ratings for TV series (season 1 and season 2)
		season1 := 1
		season2 := 2
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float64(5), &season1, tokenOwnerUser)
		_ = addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float64(8), &season2, tokenOwnerUser)

		// Delete the entire rating
		respDeleted := deleteRating(t, ratingToDelete.Id, tokenOwnerUser)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Rating with id %s deleted successfully", ratingToDelete.Id), respBody.Message)

		// DB assertion: rating should not exist
		var n int
		require.NoError(t, testPool.QueryRow(context.Background(),
			"SELECT count(*) FROM ratings WHERE id = $1", ratingToDelete.Id).Scan(&n))
		require.Zero(t, n, "Expected rating to be deleted from database")
	})
}

// TestRatingsAreGroupScoped covers the invariant that a rating's identity is
// (user, title, group), exercised end to end through the API.
//
// Its centrepiece is the leak regression subtest. Before ratings carried a
// group, the group-titles read path ran `WHERE title_id = ANY($1::text[])` with
// no group filter at all, so one group's title detail shipped every rating any
// user in the system had left on that title.
func TestRatingsAreGroupScoped(t *testing.T) {
	t.Run("The same user rates the same title in two groups and both ratings persist", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupA := float64(7)
		noteGroupB := float64(3)

		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.movie.ID, noteGroupA, nil, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.movie.ID, noteGroupB, nil, world.token)

		require.NotEqual(t, ratingGroupA.Id, ratingGroupB.Id,
			"Expected rating the same title in a second group to create a distinct rating, not reuse the first")
		require.Equal(t, world.groupA.Id, ratingGroupA.GroupId,
			"Expected the rating created in group A to report group A")
		require.Equal(t, world.groupB.Id, ratingGroupB.GroupId,
			"Expected the rating created in group B to report group B")

		// Database assertion: two rows, one per group, each carrying its own note.
		ratingGroupADb := getRating(t, ratingGroupA.Id)
		require.Equal(t, world.groupA.Id, ratingGroupADb.GroupId, "Expected group A's stored rating to carry group A's id")
		require.Equal(t, noteGroupA, ratingGroupADb.Note, "Expected group A's stored rating to keep group A's note")
		require.Equal(t, world.user.Id, ratingGroupADb.UserId, "Expected group A's stored rating to belong to the rating user")

		ratingGroupBDb := getRating(t, ratingGroupB.Id)
		require.Equal(t, world.groupB.Id, ratingGroupBDb.GroupId, "Expected group B's stored rating to carry group B's id")
		require.Equal(t, noteGroupB, ratingGroupBDb.Note, "Expected group B's stored rating to keep group B's note")

		require.Len(t, getRatingsForTitleFromDB(t, world.movie.ID), 2,
			"Expected exactly one stored rating per group for this user and title")
	})

	t.Run("Group titles show only the requesting group's ratings", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupA := float64(7)
		noteGroupB := float64(3)
		noteOtherUserGroupB := float64(10)

		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.movie.ID, noteGroupA, nil, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.movie.ID, noteGroupB, nil, world.token)
		ratingOtherUserGroupB := addRatingAndGetResult(t, world.groupB.Id, world.movie.ID, noteOtherUserGroupB, nil, world.otherToken)

		// Sanity check first: all three ratings really are stored, so a short
		// group A list below cannot be mistaken for "nothing was written".
		require.Len(t, getRatingsForTitleFromDB(t, world.movie.ID), 3,
			"Expected all three ratings on this title to be stored before asserting what each group sees")

		detailGroupA := getGroupTitleDetail(t, world.groupA.Id, world.movie.ID, world.token)
		require.Len(t, detailGroupA.GroupRatings, 1,
			"Expected group A's title detail to carry only the rating left in group A, never group B's ratings")
		require.Equal(t, ratingGroupA.Id, detailGroupA.GroupRatings[0].Id,
			"Expected group A's title detail to carry the rating created in group A")
		require.Equal(t, world.groupA.Id, detailGroupA.GroupRatings[0].GroupId,
			"Expected every rating listed for group A to be attributed to group A")
		require.Equal(t, noteGroupA, detailGroupA.GroupRatings[0].Note,
			"Expected group A's title detail to carry group A's note")

		detailGroupB := getGroupTitleDetail(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, detailGroupB.GroupRatings, 2,
			"Expected group B's title detail to carry both of its own members' ratings and nothing from group A")

		listedIds := []string{}
		listedNotes := []float64{}
		for _, rating := range detailGroupB.GroupRatings {
			require.Equal(t, world.groupB.Id, rating.GroupId,
				"Expected every rating listed for group B to be attributed to group B")
			listedIds = append(listedIds, rating.Id)
			listedNotes = append(listedNotes, rating.Note)
		}
		require.ElementsMatch(t, []string{ratingGroupB.Id, ratingOtherUserGroupB.Id}, listedIds,
			"Expected group B's title detail to list exactly the two ratings left in group B")
		require.ElementsMatch(t, []float64{noteGroupB, noteOtherUserGroupB}, listedNotes,
			"Expected group B's title detail to carry group B's notes")
	})

	t.Run("Rating a title in a second group is allowed but rating it twice in the same group still returns 409", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		_ = addRatingAndGetResult(t, world.groupA.Id, world.movie.ID, float64(7), nil, world.token)

		// A rating in another group is a different fact, not a duplicate.
		respSecondGroup := addRating(t, ratings.NewRating{
			GroupId: world.groupB.Id,
			TitleId: world.movie.ID,
			Note:    float64(3),
		}, world.token)
		defer respSecondGroup.Body.Close()
		require.Equal(t, http.StatusCreated, respSecondGroup.StatusCode,
			"Expected rating a title already rated in another group to be allowed")

		// A second rating in the same group still is a duplicate.
		respSameGroup := addRating(t, ratings.NewRating{
			GroupId: world.groupA.Id,
			TitleId: world.movie.ID,
			Note:    float64(9),
		}, world.token)
		defer respSameGroup.Body.Close()
		require.Equal(t, http.StatusConflict, respSameGroup.StatusCode,
			"Expected a second rating for the same title in the same group to still conflict")

		var respSameGroupBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respSameGroup.Body).Decode(&respSameGroupBody),
			"error decoding the conflict response body")
		require.Contains(t, respSameGroupBody.ErrorMessage, ratings.ErrRatingAlreadyExists.Error()[1:],
			"Expected the same-group duplicate to report the rating-already-exists error")

		require.Len(t, getRatingsForTitleFromDB(t, world.movie.ID), 2,
			"Expected the rejected duplicate to leave exactly the two group-scoped ratings")
	})

	t.Run("Updating a rating leaves the other group's rating untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupA := float64(7)
		noteGroupB := float64(3)
		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.movie.ID, noteGroupA, nil, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.movie.ID, noteGroupB, nil, world.token)

		updatedNote := float64(1)
		respUpdated := updateRating(t, ratings.UpdateRatingRequest{Note: updatedNote}, ratingGroupA.Id, world.token)
		defer respUpdated.Body.Close()
		require.Equal(t, http.StatusOK, respUpdated.StatusCode, "Expected group A's rating to be updated")

		var respUpdatedBody ratings.Rating
		require.NoError(t, json.NewDecoder(respUpdated.Body).Decode(&respUpdatedBody), "error decoding the updated rating")
		require.Equal(t, updatedNote, respUpdatedBody.Note, "Expected the updated rating to carry the new note")
		require.Equal(t, world.groupA.Id, respUpdatedBody.GroupId, "Expected updating a rating not to move it to another group")

		ratingGroupBDb := getRating(t, ratingGroupB.Id)
		require.Equal(t, noteGroupB, ratingGroupBDb.Note,
			"Expected group B's rating to be untouched by a PATCH aimed at group A's rating")
		require.Equal(t, world.groupB.Id, ratingGroupBDb.GroupId, "Expected group B's rating to stay attributed to group B")

		detailGroupA := getGroupTitleDetail(t, world.groupA.Id, world.movie.ID, world.token)
		require.Len(t, detailGroupA.GroupRatings, 1, "Expected group A to still show exactly its own rating")
		require.Equal(t, updatedNote, detailGroupA.GroupRatings[0].Note, "Expected group A to show the updated note")

		detailGroupB := getGroupTitleDetail(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, detailGroupB.GroupRatings, 1, "Expected group B to still show exactly its own rating")
		require.Equal(t, noteGroupB, detailGroupB.GroupRatings[0].Note, "Expected group B to still show its original note")
	})

	t.Run("Deleting a rating leaves the other group's rating untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupB := float64(3)
		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.movie.ID, float64(7), nil, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.movie.ID, noteGroupB, nil, world.token)

		respDeleted := deleteRating(t, ratingGroupA.Id, world.token)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode, "Expected group A's rating to be deleted")

		require.Zero(t, countRatingsWithId(t, ratingGroupA.Id), "Expected group A's rating to be gone from the database")
		require.Equal(t, 1, countRatingsWithId(t, ratingGroupB.Id),
			"Expected group B's rating to survive a DELETE aimed at group A's rating")

		detailGroupA := getGroupTitleDetail(t, world.groupA.Id, world.movie.ID, world.token)
		require.Empty(t, detailGroupA.GroupRatings, "Expected group A to show no ratings after deleting its only one")

		detailGroupB := getGroupTitleDetail(t, world.groupB.Id, world.movie.ID, world.token)
		require.Len(t, detailGroupB.GroupRatings, 1, "Expected group B to still show its own rating")
		require.Equal(t, ratingGroupB.Id, detailGroupB.GroupRatings[0].Id, "Expected group B to still show its own rating row")
		require.Equal(t, noteGroupB, detailGroupB.GroupRatings[0].Note, "Expected group B's note to be unchanged")
	})
}

// TestRatingsForTVSeriesAreGroupScoped exercises the per-season rating flow with
// two groups in play. The add path resolves an existing rating by
// (user, title, group); without the group predicate that lookup matches one row
// per group and silently resolves to an arbitrary one.
func TestRatingsForTVSeriesAreGroupScoped(t *testing.T) {
	season1 := 1
	season2 := 2

	t.Run("Season ratings for the same series stay independent in each group", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupASeason1 := float64(4)
		noteGroupBSeason1 := float64(10)
		noteGroupBSeason2 := float64(8)

		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.tvSeries.ID, noteGroupASeason1, &season1, world.token)
		ratingGroupBFirst := addRatingAndGetResult(t, world.groupB.Id, world.tvSeries.ID, noteGroupBSeason1, &season1, world.token)
		ratingGroupBSecond := addRatingAndGetResult(t, world.groupB.Id, world.tvSeries.ID, noteGroupBSeason2, &season2, world.token)

		require.NotEqual(t, ratingGroupA.Id, ratingGroupBFirst.Id,
			"Expected the series rating in each group to be a distinct row")
		require.Equal(t, ratingGroupBFirst.Id, ratingGroupBSecond.Id,
			"Expected both group B seasons to land on the same group B rating")

		ratingGroupADb := getRating(t, ratingGroupA.Id)
		require.Equal(t, world.groupA.Id, ratingGroupADb.GroupId, "Expected group A's series rating to carry group A's id")
		require.NotNil(t, ratingGroupADb.SeasonsRatings, "Expected group A's series rating to hold season ratings")
		require.Len(t, *ratingGroupADb.SeasonsRatings, 1,
			"Expected group A's series rating to hold only the season rated in group A")
		require.Equal(t, noteGroupASeason1, ratingGroupADb.Note,
			"Expected group A's overall note to be the mean of the seasons rated in group A only")

		ratingGroupBDb := getRating(t, ratingGroupBFirst.Id)
		require.Equal(t, world.groupB.Id, ratingGroupBDb.GroupId, "Expected group B's series rating to carry group B's id")
		require.NotNil(t, ratingGroupBDb.SeasonsRatings, "Expected group B's series rating to hold season ratings")
		require.Len(t, *ratingGroupBDb.SeasonsRatings, 2, "Expected group B's series rating to hold both seasons rated in group B")
		require.Equal(t, (noteGroupBSeason1+noteGroupBSeason2)/2, ratingGroupBDb.Note,
			"Expected group B's overall note to be the mean of the seasons rated in group B only")

		detailGroupA := getGroupTitleDetail(t, world.groupA.Id, world.tvSeries.ID, world.token)
		require.Len(t, detailGroupA.GroupRatings, 1, "Expected group A to show only its own series rating")
		require.Equal(t, world.groupA.Id, detailGroupA.GroupRatings[0].GroupId,
			"Expected group A's series rating to be attributed to group A")
		require.Equal(t, noteGroupASeason1, detailGroupA.GroupRatings[0].Note, "Expected group A to show group A's overall note")
		require.NotNil(t, detailGroupA.GroupRatings[0].SeasonsRatings, "Expected group A's listed rating to carry its seasons")
		require.Len(t, *detailGroupA.GroupRatings[0].SeasonsRatings, 1,
			"Expected group A's listed rating to carry only the season rated in group A")

		detailGroupB := getGroupTitleDetail(t, world.groupB.Id, world.tvSeries.ID, world.token)
		require.Len(t, detailGroupB.GroupRatings, 1, "Expected group B to show only its own series rating")
		require.Equal(t, world.groupB.Id, detailGroupB.GroupRatings[0].GroupId,
			"Expected group B's series rating to be attributed to group B")
		require.Equal(t, (noteGroupBSeason1+noteGroupBSeason2)/2, detailGroupB.GroupRatings[0].Note,
			"Expected group B to show group B's overall note")
		require.NotNil(t, detailGroupB.GroupRatings[0].SeasonsRatings, "Expected group B's listed rating to carry its seasons")
		require.Len(t, *detailGroupB.GroupRatings[0].SeasonsRatings, 2,
			"Expected group B's listed rating to carry both seasons rated in group B")
	})

	t.Run("Rating a season already rated in the same group returns 409 and leaves the other group untouched", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		noteGroupASeason1 := float64(4)
		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.tvSeries.ID, noteGroupASeason1, &season1, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.tvSeries.ID, float64(10), &season1, world.token)

		respDuplicate := addRating(t, ratings.NewRating{
			GroupId: world.groupB.Id,
			TitleId: world.tvSeries.ID,
			Note:    float64(6),
			Season:  &season1,
		}, world.token)
		defer respDuplicate.Body.Close()
		require.Equal(t, http.StatusConflict, respDuplicate.StatusCode,
			"Expected rating the same season twice in the same group to still conflict")

		var respDuplicateBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDuplicate.Body).Decode(&respDuplicateBody),
			"error decoding the season conflict response body")
		require.Contains(t, respDuplicateBody.ErrorMessage, ratings.ErrSeasonRatingAlreadyExists.Error()[1:],
			"Expected the same-group season duplicate to report the season-rating-already-exists error")

		ratingGroupADb := getRating(t, ratingGroupA.Id)
		require.Equal(t, noteGroupASeason1, ratingGroupADb.Note,
			"Expected group A's series rating to be untouched by a rejected group B insert")
		require.Len(t, *ratingGroupADb.SeasonsRatings, 1, "Expected group A's seasons to be untouched")

		ratingGroupBDb := getRating(t, ratingGroupB.Id)
		require.Equal(t, float64(10), ratingGroupBDb.Note, "Expected group B's series rating to keep its original note")
	})

	t.Run("Deleting a season rating leaves the other group's series rating intact", func(t *testing.T) {
		resetDB(t)
		world := setupTwoGroups(t)

		ratingGroupA := addRatingAndGetResult(t, world.groupA.Id, world.tvSeries.ID, float64(4), &season1, world.token)
		ratingGroupB := addRatingAndGetResult(t, world.groupB.Id, world.tvSeries.ID, float64(10), &season1, world.token)
		_ = addRatingAndGetResult(t, world.groupB.Id, world.tvSeries.ID, float64(8), &season2, world.token)

		// Season 1 is group A's only season, so deleting it removes the whole
		// group A rating and must not reach group B's season 1.
		respDeleted := deleteRatingSeason(t, ratingGroupA.Id, world.token, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode, "Expected group A's season rating to be deleted")

		require.Zero(t, countRatingsWithId(t, ratingGroupA.Id),
			"Expected group A's series rating to be removed once its last season was deleted")

		ratingGroupBDb := getRating(t, ratingGroupB.Id)
		require.Equal(t, world.groupB.Id, ratingGroupBDb.GroupId, "Expected group B's series rating to survive")
		require.NotNil(t, ratingGroupBDb.SeasonsRatings, "Expected group B's series rating to still hold season ratings")
		require.Len(t, *ratingGroupBDb.SeasonsRatings, 2,
			"Expected group B to keep both of its seasons after group A's season was deleted")

		detailGroupA := getGroupTitleDetail(t, world.groupA.Id, world.tvSeries.ID, world.token)
		require.Empty(t, detailGroupA.GroupRatings, "Expected group A to show no series rating after deleting its last season")

		detailGroupB := getGroupTitleDetail(t, world.groupB.Id, world.tvSeries.ID, world.token)
		require.Len(t, detailGroupB.GroupRatings, 1, "Expected group B to still show its own series rating")
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
	for _, title := range []models.Title{expectedTVSeriesTitle, expectedTVSeriesTitle2} {
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
	ratingSeason1 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float64(5), &season1, tokenOwnerUser)
	ratingSeason2 := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float64(8), &season2, tokenOwnerUser)
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
		require.Equal(t, float64(8), season2RatingDb.Rating)
		require.NotEmpty(t, season2RatingDb.AddedAt)
		require.NotEmpty(t, season2RatingDb.UpdatedAt)
		// Overall rating should be recalculated (only season 2 remains)
		require.Equal(t, float64(8), ratingDb.Note)
	})

	t.Run("Deleting last season should delete the whole rating document", func(t *testing.T) {
		onlySeason := 1
		ratingOnlySeason := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle2.ID, float64(7), &onlySeason, tokenOwnerUser)

		respDeleted := deleteRatingSeason(t, ratingOnlySeason.Id, tokenOwnerUser, onlySeason)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusOK, respDeleted.StatusCode)

		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Equal(t, fmt.Sprintf("Season %d from rating %s deleted successfully", onlySeason, ratingOnlySeason.Id), respBody.Message)

		// DB assertion: rating should not exist
		var n int
		require.NoError(t, testPool.QueryRow(context.Background(),
			"SELECT count(*) FROM ratings WHERE id = $1", ratingOnlySeason.Id).Scan(&n))
		require.Zero(t, n, "Expected rating to be deleted from database")
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
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})

	t.Run("Deleting a season rating that belongs to another user should return 404", func(t *testing.T) {
		// Add a rating for TV series by the owner
		season1 := 1
		ratingToDelete := addRatingAndGetResult(t, group.Id, expectedTVSeriesTitle.ID, float64(5), &season1, tokenOwnerUser)

		// Try to delete it with another user's token
		respDeleted := deleteRatingSeason(t, ratingToDelete.Id, tokenUserNotInGroup, season1)
		defer respDeleted.Body.Close()
		require.Equal(t, http.StatusNotFound, respDeleted.StatusCode)

		var respBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respDeleted.Body).Decode(&respBody))
		require.Contains(t, respBody.ErrorMessage, "Rating not found")
	})
}

// A note is a one-decimal value (x.x). The write path is the source of truth
// for that: a caller-supplied note with more precision is rejected outright,
// while the TV series overall note — which the service derives as the mean of
// the season ratings and no caller ever types — is rounded, because a mean of
// one-decimal values is routinely not itself one-decimal.
func TestRatingNotePrecision(t *testing.T) {
	t.Run("adding a movie rating with two decimals is rejected and writes nothing", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		resp := addRating(t, ratings.NewRating{
			GroupId: s.group.Id,
			TitleId: s.movie.ID,
			Note:    8.55,
		}, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var body api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Contains(t, body.ErrorMessage, ratings.ErrNoteTooPrecise.Error()[1:])

		require.Equal(t, 0, countRatings(t), "a rejected note must leave no rating behind")
	})

	t.Run("adding a season rating with two decimals is rejected and writes nothing", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		season := 1
		resp := addRating(t, ratings.NewRating{
			GroupId: s.group.Id,
			TitleId: s.series.ID,
			Note:    6.75,
			Season:  &season,
		}, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var body api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Contains(t, body.ErrorMessage, ratings.ErrNoteTooPrecise.Error()[1:])

		require.Equal(t, 0, countRatings(t), "a rejected season note must leave no rating behind")
	})

	t.Run("updating a movie rating to two decimals is rejected and leaves the stored note untouched", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		rating := addRatingAndGetResult(t, s.group.Id, s.movie.ID, 7.5, nil, s.token)

		resp := updateRating(t, ratings.UpdateRatingRequest{Note: 8.92}, rating.Id, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var body api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Contains(t, body.ErrorMessage, ratings.ErrNoteTooPrecise.Error()[1:])

		require.Equal(t, float64(7.5), getRating(t, rating.Id).Note,
			"a rejected update must not touch the stored note")
	})

	t.Run("updating a season rating to two decimals is rejected and leaves the stored ratings untouched", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		season := 1
		rating := addRatingAndGetResult(t, s.group.Id, s.series.ID, 7.5, &season, s.token)

		resp := updateRating(t, ratings.UpdateRatingRequest{Note: 6.75, Season: &season}, rating.Id, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var body api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Contains(t, body.ErrorMessage, ratings.ErrNoteTooPrecise.Error()[1:])

		require.Equal(t, float64(7.5), getSeasonRating(t, rating.Id, season),
			"a rejected update must not touch the stored season rating")
		require.Equal(t, float64(7.5), getRating(t, rating.Id).Note,
			"a rejected update must not touch the derived overall note")
	})

	t.Run("one-decimal and boundary notes are accepted and stored verbatim", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		// 0 and 10 are the range boundaries; 8.5 is a plain one-decimal value;
		// 6.7, 5.6, 8.7 and 0.1 are ones float64 (and, before this migration,
		// float32) cannot represent exactly in binary, which the precision
		// check has to tolerate rather than reject, and which the database
		// must now hand back exactly rather than as a widened approximation.
		for _, note := range []float64{0, 10, 8.5, 6.7, 5.6, 8.7, 0.1} {
			resetDB(t)
			s = seedRatingPrecisionScenario(t)

			rating := addRatingAndGetResult(t, s.group.Id, s.movie.ID, note, nil, s.token)
			require.Equal(t, note, rating.Note, "note %v must come back unchanged", note)
			require.Equal(t, note, getRating(t, rating.Id).Note, "note %v must be stored unchanged", note)
		}
	})

	t.Run("the JSON response renders the note as a clean decimal, not a widened float or a numeric struct", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		// Decoding into ratings.Rating (Note float64) would already catch a
		// value that came back numerically wrong, but it would not show what
		// actually went over the wire: encoding/json's shortest-round-trip
		// float64 formatting could still coincidentally decode correctly even
		// if something upstream were broken, and pgtype.Numeric marshals as
		// {"Int":...,"Exp":...} rather than erroring, which a float64 target
		// would reject at decode time rather than reveal. Asserting on the raw
		// body is what actually pins the wire format: "5.6", never
		// "5.5999999046325684" and never a struct.
		resp := addRating(t, ratings.NewRating{
			GroupId: s.group.Id,
			TitleId: s.movie.ID,
			Note:    5.6,
		}, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "error reading the raw response body")

		require.Contains(t, string(raw), `"note":5.6`,
			"the response body must render the note as a clean 5.6, got: %s", string(raw))
		require.NotContains(t, string(raw), "5.599999",
			"the response body must not carry a widened float32-origin note")
		require.NotContains(t, string(raw), `"Int"`,
			"the response body must not render the note as a pgtype.Numeric struct")
	})

	t.Run("one-decimal season notes are accepted and stored verbatim", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		season := 1
		rating := addRatingAndGetResult(t, s.group.Id, s.series.ID, 8.5, &season, s.token)
		require.Equal(t, float64(8.5), getSeasonRating(t, rating.Id, season))
		require.Equal(t, float64(8.5), getRating(t, rating.Id).Note)
	})

	t.Run("the derived TV series overall note is rounded to one decimal", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		// Since 009, this no longer isolates overallNote's own rounding: the
		// note column is NUMERIC(3,1), so Postgres rounds any excess scale on
		// write regardless of whether overallNote already did. The DB-free
		// pin for that function's own behavior is
		// internal/services/ratings.TestOverallNote; this test's job is the
		// end-to-end contract — that a caller reading the response or the row
		// sees a one-decimal mean, however it got there.
		//
		// Three one-decimal season ratings whose mean is not one-decimal:
		// (5 + 8 + 10) / 3 = 7.666…, which must be stored as 7.7.
		season1, season2, season3 := 1, 2, 3
		addRatingAndGetResult(t, s.group.Id, s.series.ID, 5, &season1, s.token)
		addRatingAndGetResult(t, s.group.Id, s.series.ID, 8, &season2, s.token)
		rating := addRatingAndGetResult(t, s.group.Id, s.series.ID, 10, &season3, s.token)

		require.Equal(t, float64(7.7), rating.Note, "the mean must be rounded in the response")
		require.Equal(t, float64(7.7), getRating(t, rating.Id).Note, "the mean must be rounded in the database")

		// Updating a season re-derives the mean, which must be rounded too:
		// (5 + 8 + 9) / 3 = 7.333… -> 7.3.
		resp := updateRating(t, ratings.UpdateRatingRequest{Note: 9, Season: &season3}, rating.Id, s.token)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.Equal(t, float64(7.3), getRating(t, rating.Id).Note,
			"the re-derived mean must be rounded after an update")

		// Deleting a season re-derives it once more: (5 + 8) / 2 = 6.5.
		respDelete := deleteRatingSeason(t, rating.Id, s.token, season3)
		defer respDelete.Body.Close()
		require.Equal(t, http.StatusOK, respDelete.StatusCode)

		require.Equal(t, float64(6.5), getRating(t, rating.Id).Note,
			"the re-derived mean must be rounded after a season delete")
	})
}

// Migration 006 repaired the rows written before the write path enforced
// one-decimal notes, back when both columns were REAL. 009 changed the
// column type to NUMERIC(3,1), which enforces the same invariant at the
// storage layer: Postgres rounds any excess scale to fit a NUMERIC column's
// declared scale on write, so insertRawRating can no longer even produce a
// stored two-decimal value the way it could before 009 — the column itself
// now does what 006's UPDATE used to have to do after the fact.
//
// 006's SQL is still exercised here, against today's schema, to pin that it
// remains a harmless no-op forever rather than something that would need to
// be dropped: every row it could touch is already at the column's declared
// scale before the statement ever runs, so it matches nothing, on the first
// run and every run after.
func TestRatingRoundingMigration(t *testing.T) {
	t.Run("is a no-op against the NUMERIC(3,1) columns 009 introduced, and stays idempotent", func(t *testing.T) {
		resetDB(t)
		s := seedRatingPrecisionScenario(t)

		// Two-decimal input, same as migration 006 once had to repair after
		// the fact — but the column now rounds it on the way in.
		insertRawRating(t, "rating-dirty-1", s.movie.ID, s.user.Id, s.group.Id, 6.75)
		insertRawRating(t, "rating-dirty-2", s.series.ID, s.user.Id, s.group.Id, 8.92)
		insertRawSeasonRating(t, "rating-dirty-2", 1, 9.05)
		insertRawSeasonRating(t, "rating-dirty-2", 2, 7.4)

		require.Equal(t, float64(6.8), getRating(t, "rating-dirty-1").Note,
			"NUMERIC(3,1) must round a two-decimal insert to one decimal on write, unaided by any migration")
		require.Equal(t, float64(8.9), getRating(t, "rating-dirty-2").Note)
		require.Equal(t, float64(9.1), getSeasonRating(t, "rating-dirty-2", 1))
		require.Equal(t, float64(7.4), getSeasonRating(t, "rating-dirty-2", 2),
			"an already one-decimal value must be left exactly as it was")

		// Running 006's Up SQL now must match nothing: every value it could
		// have repaired is already at scale 1 before it runs.
		affected := runRatingRoundingMigration(t)
		require.Equal(t, int64(0), affected, "006 must match no rows once the columns are NUMERIC(3,1)")

		affected = runRatingRoundingMigration(t)
		require.Equal(t, int64(0), affected, "a second run must match no rows either")

		require.Equal(t, float64(6.8), getRating(t, "rating-dirty-1").Note)
		require.Equal(t, float64(8.9), getRating(t, "rating-dirty-2").Note)
		require.Equal(t, float64(9.1), getSeasonRating(t, "rating-dirty-2", 1))
		require.Equal(t, float64(7.4), getSeasonRating(t, "rating-dirty-2", 2))
	})
}
