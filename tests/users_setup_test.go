package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
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
	return testStore.UserExists(context.Background(), userId)
}

func addUserAdminInDb(t *testing.T, user users.NewUserRequest) (models.User, string) {
	ctx := context.Background()

	passwordHash, err := auth.HashPassword(user.Password)
	require.NoError(t, err)

	now := time.Now()
	userDb := models.User{
		Id:           uuid.NewString(),
		Name:         user.Name,
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: passwordHash,
		Role:         models.RoleAdmin,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, testStore.AddUser(ctx, userDb))

	token := getUserToken(t, auth.LoginRequest{Username: user.Username, Email: user.Email, Password: user.Password})
	return userDb, token
}

func getUserFromDb(t *testing.T, userId string) models.User {
	u, err := testStore.GetUserById(context.Background(), userId)
	require.NoError(t, err, "error querying a user from db")
	return u
}
