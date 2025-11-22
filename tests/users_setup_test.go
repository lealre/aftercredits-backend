package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func addUser(t *testing.T, user users.NewUserRequest) users.UserResponse {
	postBody, err := json.Marshal(user)
	require.NoError(t, err)

	resp, err := http.Post(
		testServer.URL+"/users",
		"application/json",
		bytes.NewBuffer(postBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var respBody users.UserResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
	return respBody
}

// Check if a user exists directly in the database
func checkUserExists(userId string) (bool, error) {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.UsersCollection)
	count, err := coll.CountDocuments(ctx, bson.M{"_id": userId})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
