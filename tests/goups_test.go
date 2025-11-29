package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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
		user, token := addUser(t, users.NewUserRequest{
			Username: "testname",
			Password: "testpass",
		})

		// Create a group with the user
		newGroup := groups.CreateGroupRequest{
			Name: "testgroupname",
		}
		jsonData, err := json.Marshal(newGroup)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroup, err := client.Do(req)
		require.NoError(t, err)
		defer respGroup.Body.Close()
		require.Equal(t, http.StatusCreated, respGroup.StatusCode)

		var respGroupBody groups.GroupResponse
		require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&respGroupBody))
		require.Equal(t, newGroup.Name, respGroupBody.Name)
		require.Equal(t, user.Id, respGroupBody.OwnerId)
		require.Len(t, respGroupBody.Users, 1)
		require.Contains(t, respGroupBody.Users, user.Id)
		require.Empty(t, respGroupBody.Titles, "titles should be empty")
		require.NotEmpty(t, respGroupBody.CreatedAt, "createdAt should not be empty")
		require.NotEmpty(t, respGroupBody.UpdatedAt, "updatedAt should not be empty")

	})

}

func TestGroupUsers(t *testing.T) {

	t.Run("Add users to a group and retrieve them successfully", func(t *testing.T) {
		resetDB(t)

		// Create User 1
		userOne, tokenUserOne := addUser(t, users.NewUserRequest{
			Username: "testNameOne",
			Password: "testPass",
		})

		// Create User 2
		userTwo, _ := addUser(t, users.NewUserRequest{
			Username: "testNameTwo",
			Password: "testPass",
		})

		// Create a group for user one
		newGroup := groups.CreateGroupRequest{
			Name: "testgroupname",
		}
		jsonData, err := json.Marshal(newGroup)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroup, err := client.Do(req)
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
		req, err = http.NewRequest(http.MethodPost,
			testServer.URL+"/groups/"+respGroupBody.Id+"/users",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
		req.Header.Set("Content-Type", "application/json")
		client = &http.Client{}
		respGroup, err = client.Do(req)
		require.NoError(t, err)
		defer respGroup.Body.Close()
		require.Equal(t, http.StatusOK, respGroup.StatusCode)

		var respNewUserToGroupBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroup.Body).Decode(&respNewUserToGroupBody))
		require.Contains(t, respNewUserToGroupBody.Message, fmt.Sprintf("User %s added to group %s", userTwo.Id, respGroupBody.Id))

		// Check if users are in the group by querying database
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
		req, err = http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+respGroupBody.Id+"/users",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
		req.Header.Set("Content-Type", "application/json")
		client = &http.Client{}
		respGroupUsers, err := client.Do(req)
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
	_, token := addUser(t, users.NewUserRequest{
		Username: "testNameOne",
		Password: "testPass",
	})

	// Create a group for user
	newGroup := groups.CreateGroupRequest{
		Name: "testgroupname",
	}
	jsonData, err := json.Marshal(newGroup)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/groups",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	respGroup, err := client.Do(req)

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

		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups/titles",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupAddTitle, err := client.Do(req)

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

	t.Run("Get title from a group with one record successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+group.Id+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupTitles, err := client.Do(req)
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

	// Setup patch api call to be used on the next tests
	sendPatchApiCall := func(pathBody []byte) groups.GroupTitle {
		req, err := http.NewRequest(http.MethodPatch,
			testServer.URL+"/groups/"+group.Id+"/titles",
			bytes.NewBuffer(pathBody),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupSetWatched, err := client.Do(req)
		require.NoError(t, err)
		defer respGroupSetWatched.Body.Close()
		require.Equal(t, http.StatusOK, respGroupSetWatched.StatusCode)
		var resp groups.GroupTitle
		require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&resp))
		return resp
	}

	t.Run("Set title from a group as watched with watchedAt empty successfully", func(t *testing.T) {
		watched := true
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			Watched: &watched,
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Empty(t, respGroupSetWatchedBody.WatchedAt, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, len(grouDb.Titles), 1, "Expected group should have 1 title, got %d", len(grouDb.Titles))

		titleDb := grouDb.Titles[0]
		require.Equal(t, titleDb.Id, respGroupSetWatchedBody.Id, "Expected title ID in db to match response, got %s vs %s", titleDb.Id, respGroupSetWatchedBody.Id)
		require.True(t, titleDb.Watched, "Expected title Watched in db to be true")
		require.Empty(t, titleDb.WatchedAt, "Expected title WatchedAt in db to be empty")
	})

	t.Run("Set watchedAt field from a title group with watched already set as true successfully", func(t *testing.T) {
		testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			WatchedAt: &generics.FlexibleDate{
				Time: &testDate,
			},
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Equal(t, respGroupSetWatchedBody.WatchedAt, &testDate, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, 1, len(grouDb.Titles), "Expected group should have 1 title, got %d", len(grouDb.Titles))

		titleDb := grouDb.Titles[0]
		require.Equal(t, respGroupSetWatchedBody.Id, titleDb.Id, "Expected title ID in db to match response, expected: %s, got: %s", respGroupSetWatchedBody.Id, titleDb.Id)
		require.True(t, titleDb.Watched, "Expected title Watched in db to be true, got: %v", titleDb.Watched)
		require.Equal(t, &testDate, titleDb.WatchedAt, "Expected title WatchedAt in db to match testDate, expected: %v, got: %v", &testDate, titleDb.WatchedAt)
	})

	t.Run("Set watched as false should set watchedAt as empty successfully", func(t *testing.T) {
		watched := false
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			Watched: &watched,
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.False(t, respGroupSetWatchedBody.Watched, "Expected Watched to be false, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Empty(t, respGroupSetWatchedBody.WatchedAt, "Expected WatchedAt to be empty when watched is false")

		// Database assertion
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, 1, len(grouDb.Titles), "Expected group should have 1 title, got %d", len(grouDb.Titles))

		titleDb := grouDb.Titles[0]
		require.Equal(t, respGroupSetWatchedBody.Id, titleDb.Id, "Expected title ID in db to match response, expected: %s, got: %s", respGroupSetWatchedBody.Id, titleDb.Id)
		require.False(t, titleDb.Watched, "Expected title Watched in db to be false, got: %v", titleDb.Watched)
		require.Empty(t, titleDb.WatchedAt, "Expected title WatchedAt in db to be empty when watched is false")
	})

	t.Run("Setting watchedAt when watched is false in db should return 400", func(t *testing.T) {
		testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			WatchedAt: &generics.FlexibleDate{
				Time: &testDate,
			},
		})
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPatch,
			testServer.URL+"/groups/"+group.Id+"/titles",
			bytes.NewBuffer(pathBody),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupSetWatched, err := client.Do(req)
		require.NoError(t, err)
		defer respGroupSetWatched.Body.Close()
		require.Equal(t, http.StatusBadRequest, respGroupSetWatched.StatusCode)

		var respGroupSetWatchedBody api.ErrorResponse
		require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&respGroupSetWatchedBody))
		require.Contains(t, respGroupSetWatchedBody.ErrorMessage, groups.ErrUpdatingWatchedAtWhenWatchedIsFalse.Error()[1:])
	})

	t.Run("Remove title from a group successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/groups/"+group.Id+"/titles/"+expectedTitle.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupSetWatched, err := client.Do(req)
		require.NoError(t, err)
		defer respGroupSetWatched.Body.Close()
		require.Equal(t, http.StatusOK, respGroupSetWatched.StatusCode)

		var resp api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&resp))
		require.Contains(t, resp.Message, fmt.Sprintf("Title %s deleted from group %s", expectedTitle.ID, group.Id))

		// Database assertion
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, 0, len(grouDb.Titles), "Expected group should have 0 title, got %d", len(grouDb.Titles))
	})

}
