package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/stretchr/testify/require"
)

func getRating(t *testing.T, ratingId string) models.UserRating {
	ctx := context.Background()

	var r models.UserRating
	err := testPool.QueryRow(ctx,
		"SELECT id, title_id, user_id, group_id, note, created_at, updated_at FROM ratings WHERE id = $1", ratingId).
		Scan(&r.Id, &r.TitleId, &r.UserId, &r.GroupId, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	require.NoError(t, err, "error querying a rating from db")

	rows, err := testQueries.GetRatingSeasons(ctx, ratingId)
	require.NoError(t, err)
	if len(rows) > 0 {
		m := make(models.SeasonsRatings, len(rows))
		for _, row := range rows {
			m[row.Season] = models.SeasonRatingItem{Rating: row.Rating, AddedAt: row.AddedAt.Time, UpdatedAt: row.UpdatedAt.Time}
		}
		r.SeasonsRatings = &m
	}
	return r
}

// getRatingsForTitleFromDB returns every stored rating for a title regardless of
// group, ordered by group then user. The group-scoping tests need to see the
// rows the API deliberately withholds from a given group, which a group-filtered
// read path can no longer show them.
func getRatingsForTitleFromDB(t *testing.T, titleId string) []models.UserRating {
	t.Helper()
	ctx := context.Background()

	rows, err := testPool.Query(ctx,
		`SELECT id, title_id, user_id, group_id, note, created_at, updated_at
		 FROM ratings WHERE title_id = $1 ORDER BY group_id, user_id, id`, titleId)
	require.NoError(t, err, "error querying ratings from db")
	defer rows.Close()

	ratingsDb := []models.UserRating{}
	for rows.Next() {
		var r models.UserRating
		require.NoError(t, rows.Scan(&r.Id, &r.TitleId, &r.UserId, &r.GroupId, &r.Note, &r.CreatedAt, &r.UpdatedAt),
			"error scanning a rating row from db")
		ratingsDb = append(ratingsDb, r)
	}
	require.NoError(t, rows.Err(), "error iterating rating rows from db")

	return ratingsDb
}

// countRatingsWithId reports how many rating rows carry the given id, so a
// delete assertion does not have to inline a query.
func countRatingsWithId(t *testing.T, ratingId string) int {
	t.Helper()

	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT count(*) FROM ratings WHERE id = $1", ratingId).Scan(&n),
		"error counting ratings by id in db")
	return n
}

func addRating(t *testing.T, newRating ratings.NewRating, innerToken string) *http.Response {
	jsonData, err := json.Marshal(newRating)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/ratings",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func addRatingAndGetResult(t *testing.T, groupId, titleId string, note float32, season *int, token string) ratings.Rating {
	newRating := ratings.NewRating{
		GroupId: groupId,
		TitleId: titleId,
		Note:    note,
		Season:  season,
	}

	resp := addRating(t, newRating, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var rating ratings.Rating
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rating))
	return rating
}

func deleteRating(t *testing.T, ratingId, innerToken string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete,
		testServer.URL+"/ratings/"+ratingId,
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func deleteRatingSeason(t *testing.T, ratingId, innerToken string, season int) *http.Response {
	req, err := http.NewRequest(http.MethodDelete,
		testServer.URL+"/ratings/"+ratingId+"/seasons/"+strconv.Itoa(season),
		nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func updateRating(t *testing.T, ratingUppdate ratings.UpdateRatingRequest, ratingId, innerToken string) *http.Response {
	jsonData, err := json.Marshal(ratingUppdate)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPatch,
		testServer.URL+"/ratings/"+ratingId,
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}
