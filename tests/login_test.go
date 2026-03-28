package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {

	t.Run("Login with username successfully", func(t *testing.T) {
		resetDB(t)

		expectedUser := users.NewUserRequest{
			Name:     "testname",
			Username: "testuser",
			Email:    "test@email.com",
			Password: "testpass",
		}

		createdUser, _ := addUser(t, expectedUser)

		loginReq := auth.LoginRequest{
			Username: expectedUser.Username,
			Password: expectedUser.Password,
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var loginResp auth.LoginResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginResp))
		require.NotEmpty(t, loginResp.AccessToken, "access token should not be empty")
		require.Equal(t, createdUser.Id, loginResp.Id)
		require.Equal(t, expectedUser.Username, loginResp.Username)
		require.Equal(t, expectedUser.Email, loginResp.Email)
		require.Equal(t, expectedUser.Name, loginResp.Name)
		require.Empty(t, loginResp.Groups, "groups should be empty for a new user")
	})

	t.Run("Login with email successfully", func(t *testing.T) {
		resetDB(t)

		expectedUser := users.NewUserRequest{
			Name:     "testname",
			Username: "testuser",
			Email:    "test@email.com",
			Password: "testpass",
		}

		createdUser, _ := addUser(t, expectedUser)

		loginReq := auth.LoginRequest{
			Email:    expectedUser.Email,
			Password: expectedUser.Password,
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var loginResp auth.LoginResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginResp))
		require.NotEmpty(t, loginResp.AccessToken, "access token should not be empty")
		require.Equal(t, createdUser.Id, loginResp.Id)
		require.Equal(t, expectedUser.Username, loginResp.Username)
		require.Equal(t, expectedUser.Email, loginResp.Email)
	})

	t.Run("Login with wrong password should return 401", func(t *testing.T) {
		resetDB(t)

		addUser(t, users.NewUserRequest{
			Username: "testuser",
			Password: "testpass",
		})

		loginReq := auth.LoginRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusUnauthorized, errorResponse.StatusCode)
	})

	t.Run("Login with non-existent username should return 404", func(t *testing.T) {
		resetDB(t)

		loginReq := auth.LoginRequest{
			Username: "nonexistentuser",
			Password: "testpass",
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.NotEqual(t, http.StatusOK, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.NotEmpty(t, errorResponse.ErrorMessage)
	})

	t.Run("Login with missing username and email should return 400", func(t *testing.T) {
		resetDB(t)

		loginReq := auth.LoginRequest{
			Password: "testpass",
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusBadRequest, errorResponse.StatusCode)
		require.Contains(t, errorResponse.ErrorMessage, "Username or Email cannot be null")
	})

	t.Run("Login with missing password should return 400", func(t *testing.T) {
		resetDB(t)

		loginReq := auth.LoginRequest{
			Username: "testuser",
		}
		postBody, err := json.Marshal(loginReq)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusBadRequest, errorResponse.StatusCode)
		require.Contains(t, errorResponse.ErrorMessage, "password cannot be null")
	})

	t.Run("Login with invalid JSON body should return 400", func(t *testing.T) {
		resp, err := http.Post(
			testServer.URL+"/login",
			"application/json",
			bytes.NewBuffer([]byte("not valid json")),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusBadRequest, errorResponse.StatusCode)
	})
}
