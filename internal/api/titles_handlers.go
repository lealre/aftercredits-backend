package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/store"
)

func (api *API) GetTitles(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	if currentUser.Role != models.RoleAdmin {
		respondWithForbidden(w)
		return
	}

	size := generics.StringToInt(r.URL.Query().Get("size"))
	page := generics.StringToInt(r.URL.Query().Get("page"))
	orderBy := r.URL.Query().Get("orderBy")
	ascending := parseUrlQueryToBool(r.URL.Query().Get("ascending"))

	pageOfTitles, err := titles.GetPageOfTitles(api.Db, r.Context(), size, page, orderBy, ascending)
	if err != nil {
		logger.ErrorContext(r.Context(), "failed to get page of titles", "err", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch titles from database")
		return
	}

	respondWithJSON(w, http.StatusOK, pageOfTitles)
}

func (api *API) AddTitle(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	if currentUser.Role != models.RoleAdmin {
		respondWithForbidden(w)
		return
	}

	var req titles.AddTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.ErrorContext(r.Context(), "failed to decode the request body", "err", err)
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
		respondWithError(w, titles.ErrorMap[titles.ErrInvalidIMDbURL], titles.ErrInvalidIMDbURL.Error())
		return
	}
	titleID := m[1]

	if titleExists, err := titles.TitleExists(api.Db, r.Context(), titleID); titleExists {
		respondWithError(w, titles.ErrorMap[titles.ErrTitleAlreadyExists], titles.ErrTitleAlreadyExists.Error())
		return
	} else if err != nil && err != store.ErrRecordNotFound {
		logger.ErrorContext(r.Context(), "failed to check the title exists", "err", err, "title_id", titleID)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}

	title, err := titles.AddNewTitle(api.Db, api.Provider, r.Context(), titleID)
	if err != nil {
		if code, ok := titles.ErrorMap[err]; ok {
			respondWithError(w, code, err.Error())
			return
		}
		logger.ErrorContext(r.Context(), "failed to add new title", "err", err, "title_id", titleID)
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

	if currentUser.Role != models.RoleAdmin {
		respondWithForbidden(w)
		return
	}

	if ok, err := titles.TitleExists(api.Db, r.Context(), titleId); err != nil {
		logger.ErrorContext(r.Context(), "failed to check the title exists", "err", err, "title_id", titleId)
		respondWithError(w, http.StatusInternalServerError, "Database error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	err := titles.DeleteTitle(api.Db, r.Context(), titleId)
	if err != nil {
		logger.ErrorContext(r.Context(), "failed to delete title", "err", err, "title_id", titleId)
		respondWithError(w, http.StatusInternalServerError, "Database error during cascade delete")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Title deleted from database"})
}

func (api *API) SearchTitles(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	searchQuery := r.URL.Query().Get("query")
	if searchQuery == "" {
		respondWithError(w, http.StatusBadRequest, "Search query is required")
		return
	}

	// Clamp the caller-supplied limit. Unclamped, this value became the
	// capacity of a make([]T, 0, limit) inside the provider clients, so one
	// request like ?limit=2000000000 asked the runtime for a multi-gigabyte
	// allocation and OOM-killed the process. An omitted/non-positive limit
	// keeps the search default (not the page default); anything above the page
	// max is capped. Not routed through NormalizePageParams, which would
	// substitute the larger page default and quadruple provider spend.
	limit := generics.StringToInt(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = config.DefaultSearchLimit()
	}
	if maxLimit := config.MaxPageSize(); limit > maxLimit {
		limit = maxLimit
	}

	titles, err := titles.SearchTitles(api.Provider, r.Context(), searchQuery, limit)
	if err != nil {
		logger.ErrorContext(r.Context(), "failed to search titles", "err", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to search titles")
		return
	}

	respondWithJSON(w, http.StatusOK, titles)
}

// GetTitleEpisodes returns a title's episodes on demand (lazy-loaded by the UI
// when a movie modal opens). Scoped: the caller must share a group with the
// title, otherwise a stranger could walk title ids to reconstruct every group's
// watchlist. "Not yours" and "no such title" return the same 404.
func (api *API) GetTitleEpisodes(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if ok, err := titles.UserCanAccessTitle(api.Db, r.Context(), titleId, currentUser.Id); err != nil {
		logger.ErrorContext(r.Context(), "failed to check the user can access the title", "err", err, "title_id", titleId)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch episodes")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	episodes, err := titles.GetEpisodes(api.Db, r.Context(), titleId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
			return
		}
		logger.ErrorContext(r.Context(), "failed to get episodes", "err", err, "title_id", titleId)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch episodes")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]any{"episodes": episodes})
}
