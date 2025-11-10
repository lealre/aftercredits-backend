package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/groups"
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
