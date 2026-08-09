package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func getGroup(t *testing.T, groupId string) models.Group {
	ctx := context.Background()

	row, err := testQueries.GetGroupRowAnyById(ctx, groupId)
	require.NoError(t, err, "error querying a group from db")

	memberIds, err := testQueries.GetGroupMemberIds(ctx, groupId)
	require.NoError(t, err)

	titleRows, err := testQueries.GetGroupTitleRows(ctx, groupId)
	require.NoError(t, err)
	seasonRows, err := testQueries.GetGroupTitleSeasonRows(ctx, groupId)
	require.NoError(t, err)

	seasonsByTitle := map[string]models.SeasonsWatched{}
	for _, s := range seasonRows {
		if seasonsByTitle[s.TitleID] == nil {
			seasonsByTitle[s.TitleID] = models.SeasonsWatched{}
		}
		seasonsByTitle[s.TitleID][s.Season] = models.SeasonWatchedItem{
			Watched:   s.Watched,
			WatchedAt: timestamptzPtr(s.WatchedAt),
			AddedAt:   s.AddedAt.Time,
			UpdatedAt: s.UpdatedAt.Time,
		}
	}

	titles := models.GroupTitles{}
	for _, tr := range titleRows {
		var sw *models.SeasonsWatched
		if m, ok := seasonsByTitle[tr.TitleID]; ok {
			sw = &m
		}
		titles[tr.TitleID] = models.GroupTitleItem{
			TitleId:        tr.TitleID,
			TitleType:      tr.TitleType,
			SeasonsWatched: sw,
			Watched:        tr.Watched,
			AddedAt:        tr.AddedAt.Time,
			UpdatedAt:      tr.UpdatedAt.Time,
			WatchedAt:      timestamptzPtr(tr.WatchedAt),
		}
	}

	return models.Group{
		Id: row.ID, Name: row.Name, Description: row.Description, OwnerId: row.OwnerID,
		Users: memberIds, Titles: titles,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		Deleted: row.Deleted, DeletedAt: timestamptzPtr(row.DeletedAt),
	}
}

// timestamptzPtr converts a nullable pgtype.Timestamptz to *time.Time.
func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func createGroup(t *testing.T, newGroup groups.CreateGroupRequest, userToken string) groups.GroupResponse {
	jsonData, err := json.Marshal(newGroup)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroup, err := client.Do(req)

	require.NoError(t, err)
	defer respGroup.Body.Close()
	require.Equal(t, http.StatusCreated, respGroup.StatusCode)

	var group groups.GroupResponse
	require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&group))

	return group
}

func addUserToGroup(t *testing.T, addUserBody groups.AddUserToGroupRequest, groupId, token string) {
	jsonData, err := json.Marshal(addUserBody)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups/"+groupId+"/users",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroup, err := client.Do(req)
	require.NoError(t, err)

	defer respGroup.Body.Close()
	require.Equal(t, http.StatusOK, respGroup.StatusCode)
}

func addTitleToGroup(t *testing.T, newTitle groups.AddTitleToGroupRequest, token string) api.DefaultResponse {
	jsonData, err := json.Marshal(newTitle)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups/titles",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroupAddTitle, err := client.Do(req)

	require.NoError(t, err)
	defer respGroupAddTitle.Body.Close()
	require.Equal(t, http.StatusOK, respGroupAddTitle.StatusCode)

	var respGroupTitlesBody api.DefaultResponse
	require.NoError(t, json.NewDecoder(respGroupAddTitle.Body).Decode(&respGroupTitlesBody))

	return respGroupTitlesBody

}

func patchGroupTitleWatched(t *testing.T, groupId string, pathBody []byte, token string) groups.GroupTitle {
	req, err := http.NewRequest(http.MethodPatch,
		testServer.URL+"/groups/"+groupId+"/titles",
		bytes.NewBuffer(pathBody),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroupSetWatched, err := client.Do(req)
	require.NoError(t, err)
	defer respGroupSetWatched.Body.Close()
	require.Equal(t, http.StatusOK, respGroupSetWatched.StatusCode)
	var resp groups.GroupTitle
	require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&resp))
	return resp
}

func getGroupFromApi(t *testing.T, groupId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/groups/"+groupId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func updateGroupFromApi(t *testing.T, groupId string, body groups.UpdateGroupRequest, token string) *http.Response {
	jsonData, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, testServer.URL+"/groups/"+groupId, bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func deleteGroupFromApi(t *testing.T, groupId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, testServer.URL+"/groups/"+groupId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func removeUserFromGroupApi(t *testing.T, groupId, userId, token string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, testServer.URL+"/groups/"+groupId+"/users/"+userId, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// getGroupTitlesResponse calls GET /groups/{id}/titles with a raw query string
// (no leading "?") and returns the response for the caller to assert on.
func getGroupTitlesResponse(t *testing.T, groupId, query, token string) *http.Response {
	t.Helper()

	url := testServer.URL + "/groups/" + groupId + "/titles"
	if query != "" {
		url += "?" + query
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

// getGroupTitlesPage decodes a successful group-titles response.
func getGroupTitlesPage(t *testing.T, groupId, query, token string) generics.Page[groups.GroupTitleDetail] {
	t.Helper()

	resp := getGroupTitlesResponse(t, groupId, query, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var page generics.Page[groups.GroupTitleDetail]
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	return page
}

// getGroupTitlesRawBody returns the response body as a string. Needed because a
// JSON `null` and a `[]` both decode into an empty Go slice, so the difference
// is only observable before decoding.
func getGroupTitlesRawBody(t *testing.T, groupId, query, token string) string {
	t.Helper()

	resp := getGroupTitlesResponse(t, groupId, query, token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// getGroupTitleDetail returns the GroupTitleDetail for a single title from
// GET /groups/{id}/titles. The group-titles list is the read path that carries
// GroupRatings, so it is where a cross-group rating leak becomes observable.
func getGroupTitleDetail(t *testing.T, groupId, titleId, token string) groups.GroupTitleDetail {
	t.Helper()

	page := getGroupTitlesPage(t, groupId, "", token)
	for _, detail := range page.Content {
		if detail.Id == titleId {
			return detail
		}
	}

	require.FailNowf(t, "title missing from group titles page",
		"expected title %s to be listed for group %s, got %v", titleId, groupId, groupTitleIds(page))
	return groups.GroupTitleDetail{}
}

// twoGroupFixture is the world the group-scoping tests share: one user who
// belongs to two groups that both carry the same movie and the same TV series,
// plus a second user who belongs to group B only. Ratings and comments are
// group-scoped facts, so this is the smallest world in which a cross-group leak
// is observable at all.
type twoGroupFixture struct {
	user       users.UserResponse
	token      string
	otherUser  users.UserResponse
	otherToken string
	groupA     groups.GroupResponse
	groupB     groups.GroupResponse
	movie      models.Title
	tvSeries   models.Title
}

// setupTwoGroups builds the twoGroupFixture world. Callers are expected to have
// called resetDB(t) first.
func setupTwoGroups(t *testing.T) twoGroupFixture {
	t.Helper()

	user, token := addUser(t, users.NewUserRequest{
		Username: "twogroupsowner",
		Password: "testpass",
	})
	otherUser, otherToken := addUser(t, users.NewUserRequest{
		Username: "twogroupsmember",
		Password: "testpass",
	})

	groupA := createGroup(t, groups.CreateGroupRequest{Name: "group A"}, token)
	groupB := createGroup(t, groups.CreateGroupRequest{Name: "group B"}, token)

	// otherUser is a member of group B only, so everything they write is a
	// group B fact and must never surface in group A.
	addUserToGroup(t, groups.AddUserToGroupRequest{UserId: otherUser.Id}, groupB.Id, token)

	movieTitles := loadTitlesFixture(t)
	tvSeriesTitles := loadTVSeriesTitlesFixture(t)
	seedTitles(t, append(movieTitles, tvSeriesTitles...))
	movie := movieTitles[0]
	tvSeries := tvSeriesTitles[0]

	// Both groups carry both titles: the same title in two groups is exactly the
	// situation the (user, title, group) key exists to keep apart.
	for _, group := range []groups.GroupResponse{groupA, groupB} {
		for _, title := range []models.Title{movie, tvSeries} {
			addTitleToGroup(t, groups.AddTitleToGroupRequest{
				URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", title.ID),
				GroupId: group.Id,
			}, token)
		}
	}

	return twoGroupFixture{
		user:       user,
		token:      token,
		otherUser:  otherUser,
		otherToken: otherToken,
		groupA:     groupA,
		groupB:     groupB,
		movie:      movie,
		tvSeries:   tvSeries,
	}
}

// groupTitleIds reduces a page to its title ids, in order. Order is the point
// of the sorting subtests, so it is asserted explicitly rather than as a set.
func groupTitleIds(page generics.Page[groups.GroupTitleDetail]) []string {
	ids := make([]string, 0, len(page.Content))
	for _, detail := range page.Content {
		ids = append(ids, detail.Id)
	}
	return ids
}

// setGroupTitleWatched marks a group title watched (or not) with an explicit
// watchedAt, so ordering tests have deterministic values to sort on.
func setGroupTitleWatched(t *testing.T, groupId, titleId string, watched bool, watchedAt *time.Time, token string) {
	t.Helper()

	body, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
		TitleId:   titleId,
		Watched:   &watched,
		WatchedAt: &generics.FlexibleDate{Time: watchedAt},
	})
	require.NoError(t, err)
	patchGroupTitleWatched(t, groupId, body, token)
}
