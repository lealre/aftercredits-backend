package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
