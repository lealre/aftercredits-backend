package imdb

import (
	"fmt"
	"io"
	"net/http"
)

const imdbFreeApiBaseURL = "https://api.imdbapi.dev"

func FetchMovie(titleID string) ([]byte, error) {
	requestURL := fmt.Sprintf("%s/titles/%s", imdbFreeApiBaseURL, titleID)

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("non-2xx status: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
