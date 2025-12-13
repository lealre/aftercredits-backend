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
	"github.com/lealre/movies-backend/internal/mongodb"
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

		// Database assertion to check if user is added to group
		userDb := getUserFromDb(t, user.Id)
		require.NotEmpty(t, userDb, "Expected user to not be empty")
		require.NotEmpty(t, userDb.Groups, "Expected user groups to not be empty")
		require.Equal(t, 1, len(userDb.Groups), "Expected user groups to be 1, got %d", len(userDb.Groups))
		require.Contains(t, userDb.Groups, respGroupBody.Id, "Expected user groups to contain the group id")

	})

}

func TestGroupUsers(t *testing.T) {

	t.Run("Add users to a group and retrieve them successfully", func(t *testing.T) {
		resetDB(t)

		// Create User 1 (Owner)
		userOne, tokenUserOne := addUser(t, users.NewUserRequest{
			Username: "testNameOne",
			Password: "testPass",
		})

		// Create User 2 (Participant)
		userTwo, tokenUserTwo := addUser(t, users.NewUserRequest{
			Username: "testNameTwo",
			Password: "testPass",
		})

		// Create a group for user one (Owner)
		newGroup := groups.CreateGroupRequest{
			Name: "testgroupname",
		}
		respGroupBody := createGroup(t, newGroup, tokenUserOne)

		// Add User 2 (Participant) to group
		addUserToGroup := groups.AddUserToGroupRequest{
			UserId: userTwo.Id,
		}

		jsonData, err := json.Marshal(addUserToGroup)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups/"+respGroupBody.Id+"/users",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroup, err := client.Do(req)
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

		// Get users from api being user one (owner)
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

		var respGroupUserOneBody users.AllUsersResponse
		require.NoError(t, json.NewDecoder(respGroupUsers.Body).Decode(&respGroupUserOneBody))

		allUsersIds := make([]string, len(respGroupUserOneBody.Users))
		for _, user := range respGroupUserOneBody.Users {
			allUsersIds = append(allUsersIds, user.Id)
		}

		require.Contains(t, allUsersIds, userOne.Id, "group owner (userOne) is not in group response api after creation")
		require.Contains(t, allUsersIds, userTwo.Id, "user added to group not found in group response after being added")

		// Get users from api being user two
		req, err = http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+respGroupBody.Id+"/users",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserTwo)
		req.Header.Set("Content-Type", "application/json")
		client = &http.Client{}
		respGroupUsers, err = client.Do(req)
		require.NoError(t, err)
		defer respGroupUsers.Body.Close()
		require.Equal(t, http.StatusOK, respGroupUsers.StatusCode)

		var respGroupUserTwoBody users.AllUsersResponse
		require.NoError(t, json.NewDecoder(respGroupUsers.Body).Decode(&respGroupUserTwoBody))

		allUsersIds = make([]string, len(respGroupUserTwoBody.Users))
		for _, user := range respGroupUserTwoBody.Users {
			allUsersIds = append(allUsersIds, user.Id)
		}

		require.Contains(t, allUsersIds, userOne.Id, "group owner (userOne) is not in group response api after creation")
		require.Contains(t, allUsersIds, userTwo.Id, "user added to group not found in group response after being added")

		// Database assertion to check if users are in the group
		userOneDb := getUserFromDb(t, userOne.Id)
		require.NotEmpty(t, userOneDb, "Expected user to not be empty")
		require.NotEmpty(t, userOneDb.Groups, "Expected user groups to not be empty")
		require.Equal(t, 1, len(userOneDb.Groups), "Expected user groups to be 1, got %d", len(userOneDb.Groups))
		require.Contains(t, userOneDb.Groups, respGroupBody.Id, "Expected user groups to contain the group id")

		userTwoDb := getUserFromDb(t, userTwo.Id)
		require.NotEmpty(t, userTwoDb, "Expected user to not be empty")
		require.NotEmpty(t, userTwoDb.Groups, "Expected user groups to not be empty")
		require.Equal(t, 1, len(userTwoDb.Groups), "Expected user groups to be 1, got %d", len(userTwoDb.Groups))
		require.Contains(t, userTwoDb.Groups, respGroupBody.Id, "Expected user groups to contain the group id")
	})

}

func TestGroupTitles(t *testing.T) {
	resetDB(t)

	// Create User One
	_, tokenUserOne := addUser(t, users.NewUserRequest{
		Username: "testNameOne",
		Password: "testPass",
	})

	// Create User Two
	userTwo, tokenUserTwo := addUser(t, users.NewUserRequest{
		Username: "testNameTwo",
		Password: "testPass",
	})

	// Create a group for user One (Owner)
	newGroup := groups.CreateGroupRequest{
		Name: "testgroupname",
	}
	group := createGroup(t, newGroup, tokenUserOne)

	// Add user Two to group owned by user One
	userToGroup := groups.AddUserToGroupRequest{
		UserId: userTwo.Id,
	}
	addUserToGroup(t, userToGroup, group.Id, tokenUserOne)

	// User that is not part of the group
	_, tokenUserNotInGroup := addUser(t, users.NewUserRequest{
		Username: "usernotingroup",
		Password: "#Usernotingroup123",
	})

	// Load titles in database
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)
	expectedTitle := titles[0]         // Title for group owner to add
	expectedTitleTwo := titles[1]      // Title for regular user to add
	titleToUserNotInGroup := titles[2] // Different title to the user not in group to try to add

	/*
		=================== [ADD/GET] TEST OWNER USER =============
		- Test owner adding and getting one single title from group
		- Titles in group after tests: 1
		===========================================================
	*/

	t.Run("Add title to a group as group owner successfully", func(t *testing.T) {
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
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
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

		// Check if title is added to group by querying database
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

	t.Run("Get title from a group as owner successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+group.Id+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
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

	/*
		=================== [ADD/GET] TEST REGULAR USER FROM GROUP =================
		- Test group participant adding and getting another title in same the group
		- Titles in group after tests: 2
		============================================================================
	*/

	t.Run("Add title to a group as participant and not being group owner successfully", func(t *testing.T) {
		newTitle := groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitleTwo.ID),
			GroupId: group.Id,
		}

		jsonData, err := json.Marshal(newTitle)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups/titles",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserTwo)
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
			fmt.Sprintf("Title %s added to group %s", expectedTitleTwo.ID, group.Id),
			"title id and/or group id not in message response after adding a title to a group",
		)

		// Check if title is added to group by querying database
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb)
		require.NotEmpty(t, groupDb.Titles)
		require.Equal(t, len(groupDb.Titles), 2)

		var allTitlesIdsInGroup []string
		for _, title := range groupDb.Titles {
			allTitlesIdsInGroup = append(allTitlesIdsInGroup, title.Id)
		}
		require.Contains(t, allTitlesIdsInGroup, expectedTitle.ID)
		require.Contains(t, allTitlesIdsInGroup, expectedTitleTwo.ID)

	})

	t.Run("Get titles from a group as participant and not being the owner successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+group.Id+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserTwo)
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
		require.Equal(t, respGroupTitlesBody.TotalResults, 2, "Expected TotalResults to be 2, got %d", respGroupTitlesBody.TotalResults)
		require.NotEmpty(t, respGroupTitlesBody.Content, "Expected Content to not be empty")
		require.Equal(t, 2, len(respGroupTitlesBody.Content), "Expected length of Content to be 2, got %d", len(respGroupTitlesBody.Content))

		var allTitlesIds []string
		for _, title := range respGroupTitlesBody.Content {
			allTitlesIds = append(allTitlesIds, title.Id)
		}
		require.Contains(t, allTitlesIds, expectedTitle.ID)
		require.Contains(t, allTitlesIds, expectedTitleTwo.ID)
	})

	/*
		=================== [ADD/GET] TEST USER NOT FROM GROUP =====================
		- Test a user that is not from the group trying to get/add titles
		- Titles in group after tests: 2
		============================================================================
	*/

	t.Run("Add title to a group not being a participant returns 404", func(t *testing.T) {
		newTitle := groups.AddTitleToGroupRequest{
			URL:     fmt.Sprintf("https://www.imdb.com/title/%s/", titleToUserNotInGroup.ID),
			GroupId: group.Id,
		}

		jsonData, err := json.Marshal(newTitle)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/groups/titles",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserNotInGroup)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupAddTitle, err := client.Do(req)

		require.NoError(t, err)
		defer respGroupAddTitle.Body.Close()
		require.Equal(t, http.StatusNotFound, respGroupAddTitle.StatusCode)

		var respGroupTitlesBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupAddTitle.Body).Decode(&respGroupTitlesBody))
		require.Contains(t, fmt.Sprintf("Group with id %s not found", group.Id), respGroupTitlesBody.Message)

		// Check that title is not added to group by querying database
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb)
		require.NotEmpty(t, groupDb.Titles)
		require.Equal(t, len(groupDb.Titles), 2) // Should remain size two

		var allTitlesIdsInGroup []string
		for _, title := range groupDb.Titles {
			allTitlesIdsInGroup = append(allTitlesIdsInGroup, title.Id)
		}
		require.Contains(t, allTitlesIdsInGroup, expectedTitle.ID)
		require.Contains(t, allTitlesIdsInGroup, expectedTitleTwo.ID)
		require.NotContains(t, allTitlesIdsInGroup, titleToUserNotInGroup.ID)
	})

	t.Run("Get titles from a group not being a participant should return 404", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/groups/"+group.Id+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserNotInGroup)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupTitles, err := client.Do(req)
		require.NoError(t, err)

		defer respGroupTitles.Body.Close()
		require.Equal(t, http.StatusNotFound, respGroupTitles.StatusCode)

		var respGroupTitlesBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupTitles.Body).Decode(&respGroupTitlesBody))
		require.Contains(t, fmt.Sprintf("Group with id %s not found", group.Id), respGroupTitlesBody.Message)
	})

	/*
		=================== [PATCH - Watched] TEST OWNER USER =====================
		- Test a owner user to set watched titles
		- Titles in group after tests: 2
		===========================================================================
	*/

	// Setup patch api call to be used on the next tests
	sendPatchApiCall := func(pathBody []byte, innerToken string) groups.GroupTitle {
		req, err := http.NewRequest(http.MethodPatch,
			testServer.URL+"/groups/"+group.Id+"/titles",
			bytes.NewBuffer(pathBody),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+innerToken)
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

	t.Run("Set title from a group as watched with watchedAt empty as a group owner successfully", func(t *testing.T) {
		watched := true
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			Watched: &watched,
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody, tokenUserOne)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Empty(t, respGroupSetWatchedBody.WatchedAt, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb, "Expected group to not be empty")
		require.Equal(t, 2, len(groupDb.Titles), "Expected group should have 2 title, got %d", len(groupDb.Titles))

		var titleToassert mongodb.GroupTitleDb
		for _, title := range groupDb.Titles {
			if title.Id == expectedTitle.ID {
				titleToassert = title
			}
		}
		require.NotEmpty(t, titleToassert, "Expected title to be in group titles db")
		require.True(t, titleToassert.Watched, "Expected title Watched in db to be true")
		require.Empty(t, titleToassert.WatchedAt, "Expected title WatchedAt in db to be empty")
	})

	t.Run("Set watchedAt field from a title group with watched already set as true as a group owner successfully", func(t *testing.T) {
		testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			WatchedAt: &generics.FlexibleDate{
				Time: &testDate,
			},
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody, tokenUserOne)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Equal(t, respGroupSetWatchedBody.WatchedAt, &testDate, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb, "Expected group to not be empty")
		require.Equal(t, 2, len(groupDb.Titles), "Expected group should have 2 title, got %d", len(groupDb.Titles))

		var titleToassert mongodb.GroupTitleDb
		for _, title := range groupDb.Titles {
			if title.Id == expectedTitle.ID {
				titleToassert = title
			}
		}
		require.NotEmpty(t, titleToassert, "Expected title %s to be in group titles db", titleToassert.Id)
		require.True(t, titleToassert.Watched, "Expected title Watched in db to be true, got: %v", titleToassert.Watched)
		require.Equal(t, &testDate, titleToassert.WatchedAt, "Expected title WatchedAt in db to match testDate, expected: %v, got: %v", &testDate, titleToassert.WatchedAt)
	})

	t.Run("Set watched as false should set watchedAt as empty successfully", func(t *testing.T) {
		watched := false
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitle.ID,
			Watched: &watched,
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody, tokenUserOne)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitle.ID, "Expected Id to be %s, got %s", expectedTitle.ID, respGroupSetWatchedBody.Id)
		require.False(t, respGroupSetWatchedBody.Watched, "Expected Watched to be false, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Empty(t, respGroupSetWatchedBody.WatchedAt, "Expected WatchedAt to be empty when watched is false")

		// Database assertion
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb, "Expected group to not be empty")
		require.Equal(t, 2, len(groupDb.Titles), "Expected group should have 2 title, got %d", len(groupDb.Titles))

		var titleToassert mongodb.GroupTitleDb
		for _, title := range groupDb.Titles {
			if title.Id == expectedTitle.ID {
				titleToassert = title
			}
		}
		require.NotEmpty(t, titleToassert, "Expected title %s to be in group titles db", titleToassert.Id)
		require.False(t, titleToassert.Watched, "Expected title Watched in db to be false, got: %v", titleToassert.Watched)
		require.Empty(t, titleToassert.WatchedAt, "Expected title WatchedAt in db to be empty when watched is false")
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
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
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

	/*
		=================== [PATCH - Watched] TEST REGULAR USER FROM GROUP ======
		- Test a regular user to set watched titles
		- Titles in group after tests: 2
		=========================================================================
	*/

	t.Run("Set title from a group as watched with watchedAt empty not being the group owner successfully", func(t *testing.T) {
		watched := true
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitleTwo.ID,
			Watched: &watched,
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody, tokenUserTwo)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitleTwo.ID, "Expected Id to be %s, got %s", expectedTitleTwo.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Empty(t, respGroupSetWatchedBody.WatchedAt, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb, "Expected group to not be empty")
		require.Equal(t, 2, len(groupDb.Titles), "Expected group should have 2 title, got %d", len(groupDb.Titles))

		var titleToassert mongodb.GroupTitleDb
		for _, title := range groupDb.Titles {
			if title.Id == expectedTitleTwo.ID {
				titleToassert = title
			}
		}
		require.NotEmpty(t, titleToassert, "Expected title to be in group titles db")
		require.True(t, titleToassert.Watched, "Expected title Watched in db to be true")
		require.Empty(t, titleToassert.WatchedAt, "Expected title WatchedAt in db to be empty")
	})

	t.Run("Set watchedAt field from a title group with watched already set as true not being the group owner successfully", func(t *testing.T) {
		testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		pathBody, err := json.Marshal(groups.UpdateGroupTitleWatchedRequest{
			TitleId: expectedTitleTwo.ID,
			WatchedAt: &generics.FlexibleDate{
				Time: &testDate,
			},
		})
		require.NoError(t, err)
		respGroupSetWatchedBody := sendPatchApiCall(pathBody, tokenUserTwo)
		require.Equal(t, respGroupSetWatchedBody.Id, expectedTitleTwo.ID, "Expected Id to be %s, got %s", expectedTitleTwo.ID, respGroupSetWatchedBody.Id)
		require.True(t, respGroupSetWatchedBody.Watched, "Expected Watched to be true, got %v", respGroupSetWatchedBody.Watched)
		require.True(t, respGroupSetWatchedBody.AddedAt.Before(respGroupSetWatchedBody.UpdatedAt), "Expected AddedAt to be before UpdatedAt, but AddedAt: %v, UpdatedAt: %v", respGroupSetWatchedBody.AddedAt, respGroupSetWatchedBody.UpdatedAt)
		require.Equal(t, respGroupSetWatchedBody.WatchedAt, &testDate, "Expected WatchedAt to be empty when just setting watched: true")

		// Database assertion
		groupDb := getGroup(t, group.Id)
		require.NotEmpty(t, groupDb, "Expected group to not be empty")
		require.Equal(t, 2, len(groupDb.Titles), "Expected group should have 2 title, got %d", len(groupDb.Titles))

		var titleToassert mongodb.GroupTitleDb
		for _, title := range groupDb.Titles {
			if title.Id == expectedTitleTwo.ID {
				titleToassert = title
			}
		}
		require.NotEmpty(t, titleToassert, "Expected title %s to be in group titles db", titleToassert.Id)
		require.True(t, titleToassert.Watched, "Expected title Watched in db to be true, got: %v", titleToassert.Watched)
		require.Equal(t, &testDate, titleToassert.WatchedAt, "Expected title WatchedAt in db to match testDate, expected: %v, got: %v", &testDate, titleToassert.WatchedAt)
	})

	/*
		=================== [PATCH - Watched] TEST USER NOT FROM GROUP ======
		- Test a user not from goup to set watched titles
		- Titles in group after tests: 2
		=========================================================================
	*/

	t.Run("Set a title group as watched not being from the group should return 404", func(t *testing.T) {
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
		req.Header.Set("Authorization", "Bearer "+tokenUserNotInGroup)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupSetWatched, err := client.Do(req)
		require.NoError(t, err)
		defer respGroupSetWatched.Body.Close()
		require.Equal(t, http.StatusNotFound, respGroupSetWatched.StatusCode)

		var respGroupSetWatchedBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&respGroupSetWatchedBody))
		require.Contains(t, fmt.Sprintf("Group with id %s not found", group.Id), respGroupSetWatchedBody.Message)
	})

	/*
		=================== [DELETE] TEST OWNER USER ============================
		- Test a owner user to delete a title from group
		- Titles in group after tests: 1
		=========================================================================
	*/

	t.Run("Remove title from a group as a owner successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/groups/"+group.Id+"/titles/"+expectedTitle.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserOne)
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
		require.Equal(t, 1, len(grouDb.Titles), "Expected group should have 1 title, got %d", len(grouDb.Titles))
	})

	/*
		=================== [DELETE] TEST USER NOT FROM GROUP ===================
		- Test a user not from the group to delete a title that exists in group
		- Titles in group after tests: 1
		=========================================================================
	*/

	t.Run("Remove title from a group not being from group should return 404", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/groups/"+group.Id+"/titles/"+expectedTitleTwo.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserNotInGroup)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respRemoveTitle, err := client.Do(req)
		require.NoError(t, err)
		defer respRemoveTitle.Body.Close()
		require.Equal(t, http.StatusNotFound, respRemoveTitle.StatusCode)

		var respGroupTitlesBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(respRemoveTitle.Body).Decode(&respGroupTitlesBody))
		require.Contains(t, fmt.Sprintf("Group with id %s not found", group.Id), respGroupTitlesBody.Message)

		// Database assertion (Title should still be there)
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, 1, len(grouDb.Titles), "Expected group should have 1 title, got %d", len(grouDb.Titles))

		titleToAssert := grouDb.Titles[0]
		require.Equal(t, titleToAssert.Id, expectedTitleTwo.ID)
	})

	/*
		=================== [DELETE] TEST REGULAR USER FROM GROUP ===============
		- Test a regular user deleting a title from group
		- Titles in group after tests: 0
		=========================================================================
	*/

	t.Run("Remove title from a group successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/groups/"+group.Id+"/titles/"+expectedTitleTwo.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tokenUserTwo)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		respGroupSetWatched, err := client.Do(req)
		require.NoError(t, err)
		defer respGroupSetWatched.Body.Close()
		require.Equal(t, http.StatusOK, respGroupSetWatched.StatusCode)

		var resp api.DefaultResponse
		require.NoError(t, json.NewDecoder(respGroupSetWatched.Body).Decode(&resp))
		require.Contains(t, resp.Message, fmt.Sprintf("Title %s deleted from group %s", expectedTitleTwo.ID, group.Id))

		// Database assertion
		grouDb := getGroup(t, group.Id)
		require.NotEmpty(t, grouDb, "Expected group to not be empty")
		require.Equal(t, 0, len(grouDb.Titles), "Expected group should have 0 title(s), got %d", len(grouDb.Titles))
	})

}
