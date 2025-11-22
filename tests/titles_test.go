package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/stretchr/testify/require"
)

func TestAddTitles(t *testing.T) {
	t.Run("Test adding a title", func(t *testing.T) {
		resetDB(t)

		fixtureTitles := loadTitlesFixture(t)
		expectedTitle := fixtureTitles[0]

		reqBody := titles.AddTitleRequest{
			URL: fmt.Sprintf("https://www.imdb.com/title/%s/", expectedTitle.ID),
		}
		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(
			testServer.URL+"/titles",
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		var titleResp titles.Title
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&titleResp))
		require.Equal(t, titleResp.Id, expectedTitle.ID)
		require.Equal(t, titleResp.PrimaryTitle, expectedTitle.PrimaryTitle)
		require.Equal(t, titleResp.Type, expectedTitle.Type)
		require.NotNil(t, titleResp.AddedAt, "the addedAt field should not be nil when adding a title")
	})
}

func TestGetTitles(t *testing.T) {
	resetDB(t)
	var pageTitlesResponse generics.Page[titles.Title]

	t.Run("Test empty titles response", func(t *testing.T) {
		resp, err := http.Get(testServer.URL + "/titles")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.NoError(t, json.NewDecoder(resp.Body).Decode(&pageTitlesResponse))
		require.NotNil(t, pageTitlesResponse.Content)
		require.Empty(t, pageTitlesResponse.Content)
	})

	t.Run("Testing response with titles added through db client", func(t *testing.T) {
		titles := loadTitlesFixture(t)
		seedTitles(t, titles)
		resp, err := http.Get(testServer.URL + "/titles")
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
}
