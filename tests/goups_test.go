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
		user := addUser(t, users.NewUserRequest{
			Name:     "testname",
			Password: "testpass",
		})

		// Create a group with the user
		newGroup := groups.CreateGroupRequest{
			Name:    "testgroupname",
			OwnerId: user.Id,
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
		require.Contains(t, respGroupBody.Users, user.Id)
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

	t.Run("Add users to a group and retrieve them successfully", func(t *testing.T) {
		resetDB(t)

		// Create User 1
		userOne := addUser(t, users.NewUserRequest{
			Name:     "testNameOne",
			Password: "testPass",
		})

		// Create User 2
		userTwo := addUser(t, users.NewUserRequest{
			Name:     "testNameTwo",
			Password: "testPass",
		})

		// Create a group for user one
		newGroup := groups.CreateGroupRequest{
			Name:    "testgroupname",
			OwnerId: userOne.Id,
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
			UserId: userTwo.Id,
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
		require.Contains(t, respNewUserToGroupBody.Message, fmt.Sprintf("User %s added to group %s", userTwo.Id, respGroupBody.Id))

		// Check if users are in the group by querying database database
		groupDb := getGroup(t, respGroupBody.Id)
		var isUserOneInGroup, isUserTwoInGroup bool
		for _, groupUserId := range groupDb.Users {
			if userOne.Id == groupUserId {
				isUserOneInGroup = true
			}
			if userTwo.Id == groupUserId {
				isUserTwoInGroup = true
			}
		}

		require.True(t, isUserOneInGroup, "group owner (userOne) is not in group struct when querying database")
		require.True(t, isUserTwoInGroup, "user added to group not found in group users when querying database")

		// get users from api
		respGroupUsers, err := http.Get(
			testServer.URL + "/groups/" + respGroupBody.Id + "/users",
		)
		require.NoError(t, err)
		defer respGroupUsers.Body.Close()
		require.Equal(t, http.StatusOK, respGroupUsers.StatusCode)
		var respGroupUserBody users.AllUsersResponse
		require.NoError(t, json.NewDecoder(respGroupUsers.Body).Decode(&respGroupUserBody))

		allUsersIds := make([]string, len(respGroupUserBody.Users))
		for _, user := range respGroupUserBody.Users {
			allUsersIds = append(allUsersIds, user.Id)
		}

		require.Contains(t, allUsersIds, userOne.Id, "group owner (userOne) is not in group response api after creation")
		require.Contains(t, allUsersIds, userTwo.Id, "user added to group not found in group response after being added")
	})

}
