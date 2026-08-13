package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/activity"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

// activityRow is one row of the activity log, read straight from the database
// so emit-site tests do not depend on the feed API existing yet.
type activityRow struct {
	Kind      string
	GroupId   string
	ActorId   string
	ActorName string
	TitleName *string
}

// getActivityRows returns every recorded event, oldest first.
func getActivityRows(t *testing.T) []activityRow {
	t.Helper()
	rows, err := testPool.Query(context.Background(),
		`SELECT kind, group_id, actor_id, actor_name, title_name
		 FROM activity_events ORDER BY seq`)
	require.NoError(t, err, "failed to read the activity log")
	defer rows.Close()

	out := []activityRow{}
	for rows.Next() {
		var r activityRow
		require.NoError(t, rows.Scan(&r.Kind, &r.GroupId, &r.ActorId, &r.ActorName, &r.TitleName))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// countActivityRows reports how many activity_events rows exist, for the
// plug-out test, which only needs to know the log stayed empty.
func countActivityRows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT count(*) FROM activity_events").Scan(&n),
		"failed to count the activity log")
	return n
}

// seedTwoActivityTitles seeds the movie fixture and returns its first two ids,
// for feed tests that only need "a title" and "a second title" to add to a
// group without caring which ones they are.
func seedTwoActivityTitles(t *testing.T) (testTitleId, secondTestTitleId string) {
	t.Helper()

	titles := loadTitlesFixture(t)
	require.GreaterOrEqual(t, len(titles), 2, "the movie fixture must carry at least two titles")
	seedTitles(t, titles)
	return titles[0].ID, titles[1].ID
}

// seededTitleIds seeds n distinct, synthetic movie titles directly (via
// newSortableMovieTitle/seedTitles, the same helpers the sort/pagination tests
// use) and returns their ids. The committed fixture only carries 5 movies + 2
// TV series, short of what a cursor-pagination test needs, so this builds as
// many distinct titles as required instead of depending on fixture size.
func seededTitleIds(t *testing.T, n int) []string {
	t.Helper()

	titles := make([]models.Title, 0, n)
	ids := make([]string, 0, n)
	for i := range n {
		title := newSortableMovieTitle(
			fmt.Sprintf("tt988%05d", i),
			fmt.Sprintf("Activity Feed Title %d", i),
			2000+i%5,
			float64(5+i%4),
			100*(i%5),
			nil,
		)
		titles = append(titles, title)
		ids = append(ids, title.ID)
	}
	seedTitles(t, titles)
	return ids
}

// getActivityFeedResponse calls GET /activity with a raw query string
// (including the leading "?", or "" for none) and returns the response for the
// caller to assert on.
func getActivityFeedResponse(t *testing.T, token, query string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/activity"+query, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getActivityFeed decodes a successful GET /activity response.
func getActivityFeed(t *testing.T, token, query string) activity.Feed {
	t.Helper()

	resp := getActivityFeedResponse(t, token, query)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var feed activity.Feed
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&feed))
	return feed
}

// getActivityFeedRawBody returns the response body as a string. Needed because
// a JSON `null` and a `[]` both decode into an empty Go slice, so the
// difference is only observable before decoding.
func getActivityFeedRawBody(t *testing.T, token, query string) string {
	t.Helper()

	resp := getActivityFeedResponse(t, token, query)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// getActivityUnreadCountResponse calls GET /activity/unread-count and returns
// the response for the caller to assert on.
func getActivityUnreadCountResponse(t *testing.T, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/activity/unread-count", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getActivityUnreadCount decodes a successful GET /activity/unread-count
// response.
func getActivityUnreadCount(t *testing.T, token string) int64 {
	t.Helper()

	resp := getActivityUnreadCountResponse(t, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var count activity.UnreadCount
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&count))
	return count.Unread
}

// markActivityReadResponse calls POST /activity/read and returns the response
// for the caller to assert on.
func markActivityReadResponse(t *testing.T, token string, seq int64) *http.Response {
	t.Helper()

	jsonData, err := json.Marshal(activity.MarkReadRequest{Seq: seq})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/activity/read", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// markActivityRead calls POST /activity/read and asserts it succeeded.
func markActivityRead(t *testing.T, token string, seq int64) {
	t.Helper()

	resp := markActivityReadResponse(t, token, seq)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// buildGroupWithTitleAgainst registers a user, logs in, creates a group and
// adds one fixture title to it, all against baseURL rather than the shared
// testServer. It exists only for TestActivityFeedDisabled: that test builds
// its own server (the flag is read at construction time, so t.Setenv has to
// land before the server is built), which means it cannot reuse addUser,
// createGroup et al. — those all target the shared testServer.URL.
func buildGroupWithTitleAgainst(t *testing.T, baseURL string) (groupId, token string) {
	t.Helper()

	client := &http.Client{}

	registerBody, err := json.Marshal(users.NewUserRequest{Username: "offuser", Password: "pass"})
	require.NoError(t, err)
	registerResp, err := client.Post(baseURL+"/users", "application/json", bytes.NewBuffer(registerBody))
	require.NoError(t, err)
	defer registerResp.Body.Close()
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	loginBody, err := json.Marshal(auth.LoginRequest{Username: "offuser", Password: "pass"})
	require.NoError(t, err)
	loginResp, err := client.Post(baseURL+"/login", "application/json", bytes.NewBuffer(loginBody))
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var login auth.LoginResponse
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&login))

	groupBody, err := json.Marshal(groups.CreateGroupRequest{Name: "off group"})
	require.NoError(t, err)
	groupReq, err := http.NewRequest(http.MethodPost, baseURL+"/groups", bytes.NewBuffer(groupBody))
	require.NoError(t, err)
	groupReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	groupReq.Header.Set("Content-Type", "application/json")
	groupResp, err := client.Do(groupReq)
	require.NoError(t, err)
	defer groupResp.Body.Close()
	require.Equal(t, http.StatusCreated, groupResp.StatusCode)
	var group groups.GroupResponse
	require.NoError(t, json.NewDecoder(groupResp.Body).Decode(&group))

	// Any fixture id works: the off-server was built with the same
	// fixture-backed fake provider, so the fetch-and-insert-on-demand path in
	// titles.AddNewTitle covers it without needing testStore.AddTitle first.
	fixtureTitle := loadTitlesFixture(t)[0]
	titleBody, err := json.Marshal(groups.AddTitleToGroupRequest{
		URL:     "https://www.imdb.com/title/" + fixtureTitle.ID + "/",
		GroupId: group.Id,
	})
	require.NoError(t, err)
	titleReq, err := http.NewRequest(http.MethodPost, baseURL+"/groups/titles", bytes.NewBuffer(titleBody))
	require.NoError(t, err)
	titleReq.Header.Set("Authorization", "Bearer "+login.AccessToken)
	titleReq.Header.Set("Content-Type", "application/json")
	titleResp, err := client.Do(titleReq)
	require.NoError(t, err)
	defer titleResp.Body.Close()
	require.Equal(t, http.StatusOK, titleResp.StatusCode,
		"the mutating request under test must actually succeed, or the empty log proves nothing")

	return group.Id, login.AccessToken
}
