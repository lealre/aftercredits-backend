package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
)

func (api *API) CreateGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

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

	group, err := groups.CreateGroup(api.Db, r.Context(), req, currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	respondWithJSON(w, http.StatusCreated, group)
}

func (api *API) AddUserToGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	var req groups.AddUserToGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.UserId) == "" {
		respondWithError(w, http.StatusBadRequest, "UserId is required")
		return
	}

	// 1 - Get group (TODO: Call the service to do this instead of the db directly)
	if ok, err := api.Db.GroupExists(r.Context(), groupId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to group")
		return
	}

	// 2 - Check if user to be added to group exists
	if ok, err := api.Db.UserExists(r.Context(), req.UserId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", req.UserId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	}

	// 3 - Add user to group and update user group list
	err := groups.AddUserToGroup(api.Db, r.Context(), groupId, currentUser.Id, req.UserId)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to group")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("User %s added to group %s", req.UserId, groupId)})
}

func (api *API) GetTitlesFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

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

	_, err := api.Db.GetGroupById(r.Context(), groupId, currentUser.Id)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	titles, err := groups.GetTitlesFromGroup(api.Db, r.Context(), groupId, currentUser.Id, size, page, orderBy, watched, ascending)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while getting titles from group")
		return
	}

	respondWithJSON(w, http.StatusOK, titles)
}

func (api *API) GetUsersFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	_, err := api.Db.GetGroupById(r.Context(), groupId, currentUser.Id)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			logger.Printf("Group with id %s not found", groupId)
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
			return
		}
		logger.Printf("ERROR : %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to group")
		return
	}

	groupUsers, err := groups.GetUsersFromGroup(api.Db, r.Context(), groupId, currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while getting users from group")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: groupUsers})
}

func (api *API) AddTitleToGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

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
	if ok, err := api.Db.GroupExists(r.Context(), groupId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to group")
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

	err = groups.AddTitleToGroup(api.Db, r.Context(), groupId, titleID, currentUser.Id)
	if err != nil {
		if errors.Is(err, groups.ErrTitleAlreadyInGroup) {
			respondWithError(w, http.StatusBadRequest, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error adding title to group")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Title %s added to group %s", titleID, groupId)})
}

func (api *API) UpdateGroupTitleWatched(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	if ok, err := api.Db.GroupExists(r.Context(), groupId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to group")
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

	groupTitle, err := groups.UpdateGroupTitleWatched(api.Db, r.Context(), groupId, req.TitleId, currentUser.Id, req.Watched, req.WatchedAt)
	if err != nil {
		if err == groups.ErrUpdatingWatchedAtWhenWatchedIsFalse {
			respondWithError(w, http.StatusBadRequest, formatErrorMessage(err))
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error updating group title watched")
		return
	}

	respondWithJSON(w, http.StatusOK, groupTitle)
}

func (api *API) DeleteTitleFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("groupId")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	if ok, err := api.Db.GroupExists(r.Context(), groupId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "An unexpected error occurred")
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), titleId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking title")
		return
	}

	err := groups.RemoveTitleFromGroup(api.Db, r.Context(), groupId, titleId, currentUser.Id)
	if err != nil {
		if errors.Is(err, groups.ErrTitleNotInGroup) {
			respondWithError(w, http.StatusBadRequest, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error removing title from group")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Title %s deleted from group %s", titleId, groupId)})
}
