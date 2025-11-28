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
			Username: "testuser",
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

	t.Run("Adding a user with validation cases", func(t *testing.T) {
		resetDB(t)

		firstUser := users.NewUserRequest{
			Username: "testname",
			Email:    "test@email.com",
			Password: "testpass",
		}

		cases := []struct {
			user              users.NewUserRequest
			apiError          error
			stausCodeExpected int
			testErrorMessage  string
		}{
			{
				user: users.NewUserRequest{
					Username: firstUser.Username,
					Password: "testpass",
				},
				apiError:          users.ErrCredentialsAlreadyExists,
				stausCodeExpected: http.StatusConflict,
				testErrorMessage:  "Failed validating duplicated username",
			},
			{
				user: users.NewUserRequest{
					Email:    firstUser.Email,
					Password: "testpass",
				},
				apiError:          users.ErrCredentialsAlreadyExists,
				stausCodeExpected: http.StatusConflict,
				testErrorMessage:  "Failed validating duplicated email",
			},
			{
				user: users.NewUserRequest{
					Email:    "emailasstring",
					Password: "testpass",
				},
				apiError:          users.ErrInvalidEmail,
				stausCodeExpected: http.StatusBadRequest,
				testErrorMessage:  "Failed validating email format",
			},
			{
				user: users.NewUserRequest{
					Username: "1",
					Password: "testpass",
				},
				apiError:          users.ErrInvalidUsernameSize,
				stausCodeExpected: http.StatusBadRequest,
				testErrorMessage:  "Failed validating username size",
			},
			{
				user: users.NewUserRequest{
					Username: "@test&/",
					Password: "testpass",
				},
				apiError:          users.ErrInvalidUsername,
				stausCodeExpected: http.StatusBadRequest,
				testErrorMessage:  "Failed validating username special characters",
			},
			{
				user: users.NewUserRequest{
					Username: "test-name",
					Password: "1",
				},
				apiError:          users.ErrInvalidPassword,
				stausCodeExpected: http.StatusBadRequest,
				testErrorMessage:  "Failed validating password size",
			},
		}

		// Add first user
		postBody, err := json.Marshal(firstUser)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/users",
			"application/json",
			bytes.NewBuffer(postBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Run validation cases
		for _, testCase := range cases {
			newUser := testCase.user
			postBody, err := json.Marshal(newUser)
			require.NoError(t, err)

			resp, err := http.Post(
				testServer.URL+"/users",
				"application/json",
				bytes.NewBuffer(postBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, testCase.stausCodeExpected, resp.StatusCode, testCase.testErrorMessage)

			var errorResponse api.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
			require.Equal(t, testCase.stausCodeExpected, errorResponse.StatusCode, testCase.testErrorMessage)
			require.Contains(t, errorResponse.ErrorMessage, testCase.apiError.Error()[1:], testCase.testErrorMessage)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("Deleting a user successfully", func(t *testing.T) {
		resetDB(t)

		// Create a user to be deleted
		user, token := addUser(t, users.NewUserRequest{
			Name:     "testname",
			Username: "testuser",
			Password: "testpass",
		})

		// Delete the user created above
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/users/"+user.Id,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		ok, err := checkUserExists(user.Id)
		require.NoError(t, err)
		require.False(t, ok, "user should not exist after deletion")
	})

	t.Run("Attempting to delete another user's account returns 403 Forbidden", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{
			Name:     "testname",
			Username: "testuser",
			Password: "testpass",
		})

		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/users/123",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)

		var errorResponse api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResponse))
		require.Equal(t, http.StatusForbidden, errorResponse.StatusCode)
		require.Contains(t, errorResponse.ErrorMessage, api.ErrForbidden.Error()[1:])
	})
}
