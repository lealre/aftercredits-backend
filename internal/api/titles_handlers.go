package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
)

func (api *API) GetTitles(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	size := generics.StringToInt(r.URL.Query().Get("size"))
	page := generics.StringToInt(r.URL.Query().Get("page"))
	orderBy := r.URL.Query().Get("orderBy")
	ascending := parseUrlQueryToBool(r.URL.Query().Get("ascending"))
	watched := parseUrlQueryToBool(r.URL.Query().Get("watched"))

	pageOfTitles, err := titles.GetPageOfTitles(api.Db, r.Context(), size, page, orderBy, watched, ascending, nil)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch titles from database")
		return
	}

	respondWithJSON(w, http.StatusOK, pageOfTitles)
}

func (api *API) AddTitle(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var req titles.AddTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.URL == "" {
		respondWithError(w, http.StatusBadRequest, "Imdb url is required")
		return
	}

	// Accept URLs like https://www.imdb.com/title/tt8009428/ and extract the ID (tt...)
	re := regexp.MustCompile(`^https?://(?:www\.)?imdb\.com/title/(tt[0-9]+)/?`)
	m := re.FindStringSubmatch(req.URL)
	if len(m) != 2 {
		respondWithError(w, http.StatusBadRequest, "Invalid IMDb title URL")
		return
	}
	titleID := m[1]

	if titleExists, err := api.Db.TitleExists(r.Context(), titleID); titleExists {
		respondWithError(w, http.StatusBadRequest, "Title already added")
		return
	} else if err != nil && err != mongodb.ErrRecordNotFound {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}

	title, err := titles.AddNewTitle(api.Db, r.Context(), titleID)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error adding title")
		return
	}

	respondWithJSON(w, http.StatusCreated, title)
}

func (api *API) GetTitleRatings(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), titleId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	// Get all ratings for this title
	titleRatings, err := ratings.GetRatingsByTitleId(api.Db, r.Context(), titleId)
	if err != nil {
		logger.Printf("ERROR: - %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error while getting ratings")
		return
	}

	if len(titleRatings) == 0 {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("No ratings found for title with id %s", titleId))
		return
	}

	allRatings := ratings.AllRatingsFromTitle{Ratings: titleRatings}
	respondWithJSON(w, http.StatusOK, allRatings)
}

func (api *API) SetWatched(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	var req titles.SetWatchedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), titleId); err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	updatedTitle, err := titles.UpdateTitleWatchedProperties(api.Db, r.Context(), titleId, req)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error while updating title")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedTitle)
}

func (api *API) DeleteTitle(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), titleId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	deletedRatingsCount, err := titles.CascadeDeleteTitle(api.Db, r.Context(), titleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error during cascade delete")
		return
	}

	if deletedRatingsCount > 0 {
		respondWithJSON(w, http.StatusOK, fmt.Sprintf("Title and %d ratings/comments deleted from database", deletedRatingsCount))
	} else {
		respondWithJSON(w, http.StatusOK, "Title deleted from database")
	}
}
