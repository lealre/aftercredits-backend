package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestCreateGroup(t *testing.T) {

	t.Run("Create a group successfully", func(t *testing.T) {
		resetDB(t)

		// Create a new user
		respUser := addUser(t, users.NewUserRequest{
			Name:     "testname",
			Password: "testpass",
		})
		defer respUser.Body.Close()
		require.Equal(t, http.StatusCreated, respUser.StatusCode)
		var respUserBody users.UserResponse
		require.NoError(t, json.NewDecoder(respUser.Body).Decode(&respUserBody))

		// Create a group with the user
		newGroup := groups.CreateGroupRequest{
			Name:    "testgroupname",
			OwnerId: respUserBody.Id,
		}
		jsonData, err := json.Marshal(newGroup)
		require.NoError(t, err)
		respGroup, err := http.Post(
			testServer.URL+"/groups",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		defer respGroup.Body.Close()
		require.Equal(t, http.StatusCreated, respGroup.StatusCode)

		var respGroupBody groups.GroupResponse
		require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&respGroupBody))
		require.Equal(t, newGroup.Name, respGroupBody.Name)
		require.Equal(t, newGroup.OwnerId, respGroupBody.OwnerId)
		require.Len(t, respGroupBody.Users, 1)
		require.Contains(t, respGroupBody.Users, respUserBody.Id)
		require.Empty(t, respGroupBody.Titles, "titles should be empty")
		require.NotEmpty(t, respGroupBody.CreatedAt, "createdAt should not be empty")
		require.NotEmpty(t, respGroupBody.UpdatedAt, "updatedAt should not be empty")

	})

}
