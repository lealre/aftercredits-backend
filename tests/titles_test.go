package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/stretchr/testify/require"
)

func TestAddTitlesAdmin(t *testing.T) {
	t.Run("Test adding a title as admin sucessfully", func(t *testing.T) {
		resetDB(t)

		_, token := addUserAdminInDb(t, users.NewUserRequest{
			Username: "usertest",
			Password: "#Usertest1234",
		})

		fixtureTitles := loadTitlesFixture(t)
		expectedTitle := fixtureTitles[0]

		reqBody := titles.AddTitleRequest{
			URL: fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		}
		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/titles",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var titleResp titles.Title
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&titleResp))
		require.Equal(t, expectedTitle.ID, titleResp.Id)
		require.Equal(t, expectedTitle.PrimaryTitle, titleResp.PrimaryTitle)
		require.Equal(t, expectedTitle.Type, titleResp.Type)
		require.NotNil(t, titleResp.AddedAt, "the addedAt field should not be nil when adding a title")
	})

	t.Run("Test adding a title as regular user should return 403", func(t *testing.T) {
		resetDB(t)

		_, token := addUser(t, users.NewUserRequest{
			Username: "usertest",
			Password: "#Usertest1234",
		})

		fixtureTitles := loadTitlesFixture(t)
		expectedTitle := fixtureTitles[0]

		reqBody := titles.AddTitleRequest{
			URL: fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		}
		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost,
			testServer.URL+"/titles",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)

		var titleResp api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&titleResp))
		require.Contains(t, titleResp.ErrorMessage, api.ErrForbidden.Error()[1:])
	})
}

func TestGetTitlesAdmin(t *testing.T) {
	resetDB(t)
	var pageTitlesResponse generics.Page[titles.Title]
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)

	_, token := addUserAdminInDb(t, users.NewUserRequest{
		Username: "adminusertest",
		Password: "#Usertest1234",
	})

	t.Run("Test getting titles as admin sucessfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pageTitlesResponse))
		require.NotNil(t, pageTitlesResponse.Content)
		require.NotEmpty(t, pageTitlesResponse.Content)
		require.Equal(t, len(pageTitlesResponse.Content), len(titles))
		require.Equal(t, pageTitlesResponse.Size, 20)
		require.Equal(t, pageTitlesResponse.TotalResults, len(titles))
		require.Equal(t, pageTitlesResponse.TotalPages, 1)
		require.Equal(t, pageTitlesResponse.Page, 1)

		responseTitlesIds := make([]string, len(pageTitlesResponse.Content))
		responseTitlesNames := make([]string, len(pageTitlesResponse.Content))
		for i, responseTitle := range pageTitlesResponse.Content {
			responseTitlesIds[i] = responseTitle.Id
			responseTitlesNames[i] = responseTitle.PrimaryTitle
		}

		for _, title := range titles {
			require.Contains(t, responseTitlesIds, title.ID)
			require.Contains(t, responseTitlesNames, title.PrimaryTitle)
		}
	})

	t.Run("Test getting titles as regular user should return 403", func(t *testing.T) {
		_, token := addUser(t, users.NewUserRequest{
			Username: "usertest",
			Password: "#Usertest1234",
		})

		req, err := http.NewRequest(http.MethodGet,
			testServer.URL+"/titles",
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)

		var titleResp api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&titleResp))
		require.Contains(t, titleResp.ErrorMessage, api.ErrForbidden.Error()[1:])
	})
}

func TestDeleteTitlesAdmin(t *testing.T) {
	resetDB(t)
	titles := loadTitlesFixture(t)
	seedTitles(t, titles)

	t.Run("Test deleting a title admin sucessfully", func(t *testing.T) {
		titleToDelete := titles[0]

		_, token := addUserAdminInDb(t, users.NewUserRequest{
			Username: "adminusertest",
			Password: "#Usertest1234",
		})

		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/titles/"+titleToDelete.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var respBody api.DefaultResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		require.Contains(t, "Title deleted from database", respBody.Message)

		// assert titles is not in database
		allTitlesInDb := getTitles(t)
		allTitlesIds := make([]string, len(allTitlesInDb))
		for _, title := range allTitlesInDb {
			allTitlesIds = append(allTitlesIds, title.ID)
		}

		require.NotContains(t, allTitlesIds, titleToDelete.ID)
	})

	t.Run("Test deleting a title as regular user should return 403", func(t *testing.T) {
		titleToDelete := titles[0]

		_, token := addUser(t, users.NewUserRequest{
			Username: "usertest",
			Password: "#Usertest1234",
		})

		req, err := http.NewRequest(http.MethodDelete,
			testServer.URL+"/titles/"+titleToDelete.ID,
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)

		var titleResp api.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&titleResp))
		require.Contains(t, titleResp.ErrorMessage, api.ErrForbidden.Error()[1:])
	})
}
