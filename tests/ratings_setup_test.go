package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/users"
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

// ratingPrecisionScenario is the setup every note-precision case needs: a user
// who owns a group holding one movie and one TV series. Each precision subtest
// starts from resetDB, so the setup is per-subtest rather than shared.
type ratingPrecisionScenario struct {
	user   users.UserResponse
	token  string
	group  groups.GroupResponse
	movie  models.Title
	series models.Title
}

func seedRatingPrecisionScenario(t *testing.T) ratingPrecisionScenario {
	t.Helper()

	user, token := addUser(t, users.NewUserRequest{
		Username: "precisionuser",
		Password: "testpass",
	})

	group := createGroup(t, groups.CreateGroupRequest{
		Name: "precision group",
	}, token)

	movieTitles := loadTitlesFixture(t)
	seedTitles(t, movieTitles)
	seriesTitles := loadTVSeriesTitlesFixture(t)
	seedTitles(t, seriesTitles)

	movie := movieTitles[0]
	series := seriesTitles[0]

	for _, titleId := range []string{movie.ID, series.ID} {
		addTitleToGroup(t, groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleId),
			GroupId: group.Id,
		}, token)
	}

	return ratingPrecisionScenario{
		user:   user,
		token:  token,
		group:  group,
		movie:  movie,
		series: series,
	}
}

// countRatings reports how many rating rows exist at all, so a rejection case
// can assert that nothing was written rather than only that the status was 400.
func countRatings(t *testing.T) int {
	t.Helper()

	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT count(*) FROM ratings").Scan(&n), "error counting ratings in db")
	return n
}

// getSeasonRating returns one stored season rating straight from the table, for
// assertions about the value that was actually persisted.
func getSeasonRating(t *testing.T, ratingId string, season int) float32 {
	t.Helper()

	var v float32
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT rating FROM rating_seasons WHERE rating_id = $1 AND season = $2",
		ratingId, strconv.Itoa(season)).Scan(&v), "error querying a season rating from db")
	return v
}

// insertRawRating writes a rating row with the given note directly, bypassing
// the service. The write path now rejects two-decimal notes, so a row in the
// state migration 006 has to repair can only be produced this way — which is
// exactly the point: these are rows that predate the enforcement.
func insertRawRating(t *testing.T, ratingId, titleId, userId, groupId string, note float32) {
	t.Helper()

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO ratings (id, title_id, user_id, group_id, note, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now(), now())`,
		ratingId, titleId, userId, groupId, note)
	require.NoError(t, err, "error inserting a raw rating row")
}

func insertRawSeasonRating(t *testing.T, ratingId string, season int, rating float32) {
	t.Helper()

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO rating_seasons (rating_id, season, rating, added_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())`,
		ratingId, strconv.Itoa(season), rating)
	require.NoError(t, err, "error inserting a raw season rating row")
}

// runRatingRoundingMigration executes the Up half of migration 006 against the
// test database and reports how many rows it changed. It reads the committed
// file rather than restating the SQL, so the test exercises the migration that
// actually ships; goose has already applied it once during TestMain, and
// re-running it here is both how the repair is tested and how its idempotency
// is proven.
//
// The row count is the point of the return value. Rounding is idempotent in
// *value* no matter how the statements are written — round(round(x)) is
// round(x) — so asserting only that the values stayed put would pass even if
// the WHERE clauses were dropped entirely. Rows-affected is what actually
// distinguishes "the second run matched nothing" from "the second run rewrote
// every row to the value it already had".
func runRatingRoundingMigration(t *testing.T) int64 {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(schemaDir, "006_round_ratings_to_one_decimal.sql"))
	require.NoError(t, err, "error reading migration 006")

	body := string(raw)
	up := strings.Index(body, "-- +goose Up")
	down := strings.Index(body, "-- +goose Down")
	require.NotEqual(t, -1, up, "migration 006 has no Up section")
	require.NotEqual(t, -1, down, "migration 006 has no Down section")

	upSection := body[up+len("-- +goose Up") : down]
	require.Contains(t, upSection, "UPDATE ratings", "migration 006 Up section did not parse as expected")
	require.Contains(t, upSection, "UPDATE rating_seasons", "migration 006 Up section did not parse as expected")

	// Statements are run one at a time so each one's rows-affected can be
	// counted; a single multi-statement Exec would only report the last tag.
	var affected int64
	var ran int
	for _, statement := range strings.Split(upSection, ";") {
		if strings.TrimSpace(stripSQLComments(statement)) == "" {
			continue
		}
		tag, err := testPool.Exec(context.Background(), statement)
		require.NoError(t, err, "error running a migration 006 Up statement")
		affected += tag.RowsAffected()
		ran++
	}
	require.Equal(t, 2, ran, "migration 006 Up should be exactly the two repair statements")

	return affected
}

func stripSQLComments(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
