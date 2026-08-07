package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/groups"
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
