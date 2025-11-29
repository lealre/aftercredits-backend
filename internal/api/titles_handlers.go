package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
)

func (api *API) GetTitles(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	if currentUser.Role != mongodb.RoleAdmin {
		respondWithForbidden(w)
		return
	}

	size := generics.StringToInt(r.URL.Query().Get("size"))
	page := generics.StringToInt(r.URL.Query().Get("page"))
	orderBy := r.URL.Query().Get("orderBy")
	ascending := parseUrlQueryToBool(r.URL.Query().Get("ascending"))

	pageOfTitles, err := titles.GetPageOfTitles(api.Db, r.Context(), size, page, orderBy, ascending, nil)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch titles from database")
		return
	}

	respondWithJSON(w, http.StatusOK, pageOfTitles)
}

func (api *API) AddTitle(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	if currentUser.Role != mongodb.RoleAdmin {
		respondWithForbidden(w)
		return
	}

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

func (api *API) DeleteTitle(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if currentUser.Role != mongodb.RoleAdmin {
		respondWithForbidden(w)
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

	err := titles.DeleteTitle(api.Db, r.Context(), titleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error during cascade delete")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Title deleted from database"})
}
