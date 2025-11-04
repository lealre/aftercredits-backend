package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestGetTitles(t *testing.T) {
	resetDB(t)
	var pageTitlesResponse generics.Page[titles.Title]

	fmt.Println("Testing empty response..")
	resp, err := http.Get(testServer.URL + "/titles")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pageTitlesResponse))
	require.NotNil(t, pageTitlesResponse.Content)
	require.Empty(t, pageTitlesResponse.Content)

	fmt.Println("Testing response with titles..")
	titles := loadFixture(t, "fixtures/titles.json")
	seedTitles(t, titles)
	resp, err = http.Get(testServer.URL + "/titles")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pageTitlesResponse))
	require.NotNil(t, pageTitlesResponse.Content)
	require.NotEmpty(t, pageTitlesResponse.Content)
	require.Equal(t, len(pageTitlesResponse.Content), len(titles))

	responseTitlesNames := make([]string, len(pageTitlesResponse.Content))
	for i, responseTitle := range pageTitlesResponse.Content {
		responseTitlesNames[i] = responseTitle.PrimaryTitle
	}

	for _, title := range titles {
		titleMap, ok := title.(bson.M)
		require.True(t, ok, "failed to cast title to bson.M")

		primaryTitle, ok := titleMap["primaryTitle"].(string)
		require.True(t, ok, "failed to get primaryTitle as string")

		require.Contains(t, responseTitlesNames, primaryTitle)
	}

}
