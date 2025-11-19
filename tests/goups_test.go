package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
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

	t.Run("Create a group with wrong user id returns 404", func(t *testing.T) {
		resetDB(t)

		unknowId := "userId"
		newGroup := groups.CreateGroupRequest{
			Name:    "testgroupname",
			OwnerId: unknowId,
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
		require.Equal(t, http.StatusNotFound, respGroup.StatusCode)

		var resp api.ErrorResponse
		require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&resp))
		require.Contains(t, resp.ErrorMessage, fmt.Sprintf("id %s not found", unknowId))

	})

}

func TestGroupUsers(t *testing.T) {

	t.Run("Get users from a group successfully", func(t *testing.T) {
		resetDB(t)

		userOne := users.NewUserRequest{
			Name:     "testNameOne",
			Password: "testPass",
		}
		userTwo := users.NewUserRequest{
			Name:     "testNameTwo",
			Password: "testPass",
		}

		// Create User 1
		respUserOne := addUser(t, userOne)
		defer respUserOne.Body.Close()
		require.Equal(t, http.StatusCreated, respUserOne.StatusCode)
		var respUserOneBody users.UserResponse
		require.NoError(t, json.NewDecoder(respUserOne.Body).Decode(&respUserOneBody))

		// Create User 2
		respUserTwo := addUser(t, userTwo)
		defer respUserOne.Body.Close()
		require.Equal(t, http.StatusCreated, respUserTwo.StatusCode)
		var respUserTwoBody users.UserResponse
		require.NoError(t, json.NewDecoder(respUserTwo.Body).Decode(&respUserTwoBody))

		// Create a group for user one
		newGroup := groups.CreateGroupRequest{
			Name:    "testgroupname",
			OwnerId: respUserOneBody.Id,
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

		// Add User 2 to group
		addUserToGroup := groups.AddUserToGroupRequest{
			UserId: respUserTwoBody.Id,
		}
		jsonData, err = json.Marshal(addUserToGroup)
		require.NoError(t, err)
		respGroup, err = http.Post(
			testServer.URL+"/groups/"+respGroupBody.Id+"/users",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		defer respGroup.Body.Close()
		require.Equal(t, http.StatusOK, respGroup.StatusCode)

		var respNewUserToGroupBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&respNewUserToGroupBody))
		require.Contains(t, respNewUserToGroupBody.Message, fmt.Sprintf("User %s added to group %s", respUserTwoBody.Id, respGroupBody.Id))

	})

}
