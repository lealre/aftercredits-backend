package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func addUser(t *testing.T, user users.NewUserRequest) (users.UserResponse, string) {

	// Add user
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

	// Get token
	authUser := auth.LoginRequest{
		Username: user.Username,
		Password: user.Password,
	}
	token := getUserToken(t, authUser)

	return respBody, token
}

func getUserToken(t *testing.T, authUser auth.LoginRequest) string {
	postBody, err := json.Marshal(authUser)
	require.NoError(t, err)

	resp, err := http.Post(
		testServer.URL+"/login",
		"application/json",
		bytes.NewBuffer(postBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var respBodyAuth auth.LoginResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBodyAuth))

	return respBodyAuth.AccessToken
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

func addUserAdminInDb(t *testing.T, user users.NewUserRequest) (mongodb.UserDb, string) {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.UsersCollection)

	passordHash, err := auth.HashPassword(user.Password)
	require.NoError(t, err)

	now := time.Now()
	userDb := mongodb.UserDb{
		Id:           primitive.NewObjectID().Hex(),
		Name:         user.Name,
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: passordHash,
		Role:         mongodb.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = coll.InsertOne(ctx, userDb)
	require.NoError(t, err)

	// Get token
	authUser := auth.LoginRequest{
		Username: user.Username,
		Email:    user.Email,
		Password: user.Password,
	}
	token := getUserToken(t, authUser)

	return userDb, token
}

func getUserFromDb(t *testing.T, userId string) mongodb.UserDb {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.UsersCollection)
	var userDb mongodb.UserDb
	err := coll.FindOne(ctx, bson.M{"_id": userId}).Decode(&userDb)
	require.NoError(t, err, "error querying a user from db")
	return userDb
}
