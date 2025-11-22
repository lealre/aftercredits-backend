package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/generics"
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

func TestGroupTitles(t *testing.T) {

	// --- TEST SETUP ----
	resetDB(t)

	// Create User 1
	userOne := addUser(t, users.NewUserRequest{
		Name:     "testNameOne",
		Password: "testPass",
	})

	// Create a group for user
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

	var group groups.GroupResponse
	require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&group))

	// Load titles in database
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)
	expectedTitle := titles[0]

	t.Run("Add one title to a group successfully", func(t *testing.T) {
		newTitle := groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
			GroupId: group.Id,
		}

		jsonData, err := json.Marshal(newTitle)
		require.NoError(t, err)
		respGroupAddTitle, err := http.Post(
			testServer.URL+"/groups/titles",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		defer respGroupAddTitle.Body.Close()
		require.Equal(t, http.StatusOK, respGroupAddTitle.StatusCode)

		var respGroupTitlesBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupAddTitle.Body).Decode(&respGroupTitlesBody))
		require.Contains(
			t,
			respGroupTitlesBody.Message,
			fmt.Sprintf("Title %s added to group %s", expectedTitle.ID, group.Id),
			"title id and/or group id not in message response after adding a title to a group",
		)

		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb)
		require.NotEmpty(t, groupDb.Titles)
		require.Equal(t, len(groupDb.Titles), 1)

		groupTitleDb := groupDb.Titles[0]
		require.Equal(t, groupTitleDb.Id, expectedTitle.ID, "group title ID should match expected title ID when adding a title to a group")
		require.NotEmpty(t, groupTitleDb.AddedAt, "AddedAt should not be empty when adding a title to a group")
		require.NotEmpty(t, groupTitleDb.UpdatedAt, "UpdatedAt should not be empty when adding a title to a group")
		require.False(t, groupTitleDb.Watched, "Watched should be false by default when adding a title to a group")
		require.Empty(t, groupTitleDb.WatchedAt, "WatchedAt should be empty by default when adding a title to a group")
	})

	t.Run("Get title from a group expecting one record successfully", func(t *testing.T) {
		respGroupTitles, err := http.Get(
			testServer.URL + "/groups/" + group.Id + "/titles",
		)
		require.NoError(t, err)
		defer respGroupTitles.Body.Close()
		require.Equal(t, http.StatusOK, respGroupTitles.StatusCode)

		var respGroupTitlesBody generics.Page[groups.GroupTitleDetail]
		require.NoError(t, json.NewDecoder(respGroupTitles.Body).Decode(&respGroupTitlesBody))
		require.Equal(t, respGroupTitlesBody.Page, 1, "Expected Page to be 1, got %d", respGroupTitlesBody.Page)
		require.Equal(t, respGroupTitlesBody.Size, 20, "Expected Size to be 20, got %d", respGroupTitlesBody.Size)
		require.Equal(t, respGroupTitlesBody.TotalPages, 1, "Expected TotalPages to be 1, got %d", respGroupTitlesBody.TotalPages)
		require.Equal(t, respGroupTitlesBody.TotalResults, 1, "Expected TotalResults to be 1, got %d", respGroupTitlesBody.TotalResults)
		require.NotEmpty(t, respGroupTitlesBody.Content, "Expected Content to not be empty")
		require.Equal(t, len(respGroupTitlesBody.Content), 1, "Expected length of Content to be 1, got %d", len(respGroupTitlesBody.Content))

		respTitle := respGroupTitlesBody.Content[0]
		require.Equal(t, respTitle.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respTitle.Id)
		require.Equal(t, respTitle.PrimaryTitle, expectedTitle.PrimaryTitle, "Expected PrimaryTitle to be %s, got %s", expectedTitle.PrimaryTitle, respTitle.PrimaryTitle)

	})

	// TODO: Add test to set title as watched and date watched
	// TODO: Add test to set title as unwatched
	// TODO: Add test to remove title from group
}
