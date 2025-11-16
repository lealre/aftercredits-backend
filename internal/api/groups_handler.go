package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
)

func (api *API) CreateGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var req groups.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		respondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}

	if req.OwnerId == "" {
		respondWithError(w, http.StatusBadRequest, "Owner id is required")
		return
	}

	if ok, err := api.Db.UserExists(r.Context(), req.OwnerId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", req.OwnerId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	}

	group, err := groups.CreateGroup(api.Db, r.Context(), req)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	respondWithJSON(w, http.StatusCreated, group)
}

func (api *API) GetTitlesFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	size := generics.StringToInt(r.URL.Query().Get("size"))
	page := generics.StringToInt(r.URL.Query().Get("page"))
	orderBy := r.URL.Query().Get("orderBy")
	ascending := parseUrlQueryToBool(r.URL.Query().Get("ascending"))
	watched := parseUrlQueryToBool(r.URL.Query().Get("watched"))

	if ok, err := api.Db.GroupExists(r.Context(), groupId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking group")
		return
	}

	titles, err := groups.GetTitlesFromGroup(api.Db, r.Context(), groupId, size, page, orderBy, watched, ascending)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while getting titles from group")
		return
	}

	respondWithJSON(w, http.StatusOK, titles)
}

func (api *API) GetUsersFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	if ok, err := api.Db.GroupExists(r.Context(), groupId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking group")
		return
	}

	groupUsers, err := groups.GetUsersFromGroup(api.Db, r.Context(), groupId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while getting users from group")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: groupUsers})
}

func (api *API) AddTitleToGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var req groups.AddTitleToGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.URL == "" {
		respondWithError(w, http.StatusBadRequest, "Imdb url is required")
		return
	}

	groupId := req.GroupId
	if ok, err := api.Db.GroupExists(r.Context(), req.GroupId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking group")
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

	// If titles id is not in the main titles collection, add it
	titleExists, err := api.Db.TitleExists(r.Context(), titleID)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}

	if !titleExists {
		logger.Printf("Title %s not found in main titles collection, adding it", titleID)
		_, err = titles.AddNewTitle(api.Db, r.Context(), titleID)
		if err != nil {
			logger.Printf("ERROR: adding new title %s: %v", titleID, err)
			respondWithError(w, http.StatusInternalServerError, "Error adding title")
			return
		}
	} else {
		logger.Printf("Title %s found in main titles collection, getting it", titleID)
		_, err = titles.GetTitleById(api.Db, r.Context(), titleID)
		if err != nil {
			logger.Printf("ERROR: getting title %s: %v", titleID, err)
			respondWithError(w, http.StatusInternalServerError, "Error getting title")
			return
		}
	}

	err = groups.AddTitleToGroup(api.Db, r.Context(), groupId, titleID)
	if err != nil {
		if errors.Is(err, groups.ErrTitleAlreadyInGroup) {
			respondWithError(w, http.StatusBadRequest, "Title already added to group")
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error adding title to group")
		return
	}

	respondWithJSON(w, http.StatusOK, fmt.Sprintf("Title %s added to group %s", titleID, groupId))
}

func (api *API) UpdateGroupTitleWatched(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	if ok, err := api.Db.GroupExists(r.Context(), groupId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking group")
		return
	}

	var req groups.UpdateGroupTitleWatchedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.TitleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), req.TitleId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", req.TitleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking title")
		return
	}

	groupTitle, err := groups.UpdateGroupTitleWatched(api.Db, r.Context(), groupId, req.TitleId, req.Watched, req.WatchedAt)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error updating group title watched")
		return
	}

	respondWithJSON(w, http.StatusOK, groupTitle)
}
