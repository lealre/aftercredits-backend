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

// addUser seeds an ordinary user directly into the store and returns a login
// token for it. It deliberately bypasses POST /users: registration is now
// admin-only (a public POST /users is a 401), and this shared helper exists to
// give a test "a user with a token", not to exercise the registration route —
// the tests that exercise that route do so explicitly. Seeding through the
// store also means fixtures with short passwords keep working, since the store
// does not apply the service-level password policy.
func addUser(t *testing.T, user users.NewUserRequest) (users.UserResponse, string) {
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
		Role:         models.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, testStore.AddUser(ctx, userDb))

	token := getUserToken(t, auth.LoginRequest{
		Username: user.Username,
		Email:    user.Email,
		Password: user.Password,
	})

	return users.MapDbUserToApiUserResponse(userDb), token
}

// seedUserInStore inserts an ordinary user directly into the shared store and
// returns the created record. Used where a test needs the user to exist but
// logs in (or acts) against a specific server instance itself. Bypasses the
// admin-gated POST /users route and the service password policy.
func seedUserInStore(t *testing.T, user users.NewUserRequest) models.User {
	t.Helper()
	passwordHash, err := auth.HashPassword(user.Password)
	require.NoError(t, err)
	now := time.Now()
	userDb := models.User{
		Id:           uuid.NewString(),
		Name:         user.Name,
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: passwordHash,
		Role:         models.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, testStore.AddUser(context.Background(), userDb))
	return userDb
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
