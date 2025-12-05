package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func getGroup(t *testing.T, groupId string) mongodb.GroupDb {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.GroupsCollection)
	var group mongodb.GroupDb
	err := coll.FindOne(ctx, bson.M{"_id": groupId}).Decode(&group)
	require.NoError(t, err, "error queryind a group from db")

	return group

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

func addTitleToGroup(t *testing.T, newTitle groups.AddTitleToGroupRequest, token string) {
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
}
