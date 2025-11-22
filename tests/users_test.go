package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddUsers(t *testing.T) {
	t.Run("Adding a user successfully", func(t *testing.T) {
		resetDB(t)

		newUser := users.NewUserRequest{
			Name:     "testname",
			Password: "testpass",
		}
		postBody, err := json.Marshal(newUser)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/users",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var respBody users.UserResponse
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)
		require.NotEmpty(t, respBody.Id, "id should not be empty")
		require.NotEmpty(t, respBody.Name, "name should not be empty")
		require.NotEmpty(t, respBody.CreatedAt, "createdAt should not be empty")
		require.NotEmpty(t, respBody.UpdatedAt, "updatedAt should not be empty")
		require.Equal(t, respBody.Name, newUser.Name, "username returned should be the same in post body")
		require.Empty(t, respBody.LastLoginAt, "lastLoginAt should be empty")
		require.Empty(t, respBody.AvatarURL, "avatarURL should be empty")
		require.Empty(t, respBody.Groups, "groups should be empty")
		require.Empty(t, respBody.Email, "email should be empty")
	})

	t.Run("Adding a user with duplicated username", func(t *testing.T) {
		resetDB(t)

		newUser := users.NewUserRequest{
			Name:     "testname",
			Password: "testpass",
		}
		postBody, err := json.Marshal(newUser)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/users",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		secondResp, err := http.Post(
			testServer.URL+"/users",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer secondResp.Body.Close()
		require.Equal(t, http.StatusBadRequest, secondResp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(secondResp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusBadRequest, errorResponse.StatusCode)
	})

}

func TestDeleteUser(t *testing.T) {
	t.Run("Deleting a user successfully", func(t *testing.T) {
		resetDB(t)

		// Create a user to be deleted
		user := addUser(t, users.NewUserRequest{
			Name:     "testname",
			Password: "testpass",
		})

		// Delete the user created above
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/users/"+user.Id,
			nil,
		)
		require.NoError(t, err)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		ok, err := checkUserExists(user.Id)
		require.NoError(t, err)
		require.False(t, ok, "user should not exist after deletion")
	})

	t.Run("Deleting a user that does not exist", func(t *testing.T) {
		resetDB(t)

		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/users/",
			nil,
		)
		require.NoError(t, err)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

}
