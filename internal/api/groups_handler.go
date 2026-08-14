package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/groups"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/lealre/movies-backend/internal/store"
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

	group, err := groups.CreateGroup(api.Db, r.Context(), req, currentUser.Id)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusCreated, group)
}

func (api *API) GetGroupById(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	group, err := groups.GetGroupById(api.Db, r.Context(), groupId, currentUser.Id)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, group)
}

func (api *API) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	var req groups.UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	group, err := groups.UpdateGroupInfo(api.Db, r.Context(), groupId, currentUser.Id, req.Name, req.Description)
	if err != nil {
		if code, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, code, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	respondWithJSON(w, http.StatusOK, group)
}

func (api *API) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	if err := groups.SoftDeleteGroup(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		if code, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, code, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Group deleted"})
}

func (api *API) RemoveUserFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	userId := r.PathValue("userId")
	if groupId == "" || userId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id and user id are required")
		return
	}

	// This version supports self-removal (leave) only; removing other members is deferred.
	if userId != currentUser.Id {
		respondWithForbidden(w)
		return
	}

	if err := groups.LeaveGroup(api.Db, r.Context(), groupId, userId); err != nil {
		if code, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, code, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Left group"})
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

	// 1 - Check the group exists for this user
	if ok, err := groups.GroupExists(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	}

	// 2 - Check if user to be added to group exists
	if ok, err := users.UserExists(api.Db, r.Context(), req.UserId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", req.UserId))
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
	titleType := r.URL.Query().Get("titleType")
	var titleTypePtr *string
	if titleType != "" {
		titleTypePtr = &titleType
	}

	// The one existence/membership guard for this endpoint — GroupExists is a
	// single EXISTS query, where loading the group would materialize every
	// title and season row. GetTitlesFromGroup deliberately does not repeat
	// it, so this must stay.
	if ok, err := groups.GroupExists(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	}

	titles, err := groups.GetTitlesFromGroup(api.Db, r.Context(), groupId, size, page, orderBy, watched, ascending, titleTypePtr)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, titles)
}

// GetTitleFromGroup serves GET /groups/{groupId}/titles/{titleId}: the
// group-scoped detail of exactly one title, in the same shape as one element of
// the GET /groups/{groupId}/titles Content array.
//
// It exists because that list is paginated, so a client holding only a title id
// — the activity feed, deep-linking a row to the title's modal — has no
// reliable way to reach the entry it wants.
func (api *API) GetTitleFromGroup(w http.ResponseWriter, r *http.Request) {
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

	// One EXISTS answers every way this request is not allowed to see the
	// title — unknown group, deleted group, caller not a member, title not in
	// the group — and they are all the same 404, so none of them tells an
	// outsider which one it was. Same guard and same message as the comments
	// routes under this path.
	if ok, err := groups.GroupContainsTitle(api.Db, r.Context(), groupId, titleId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", groupId, titleId))
		return
	}

	detail, err := groups.GetGroupTitleDetail(api.Db, r.Context(), groupId, titleId)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, detail)
}

func (api *API) GetUsersFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("id")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	// Existence/membership guard only — GroupExists is a single EXISTS query,
	// where loading the group would materialize every title and season row.
	if ok, err := groups.GroupExists(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		logger.Printf("Group with id %s not found", groupId)
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	}

	groupUsers, err := groups.GetUsersFromGroup(api.Db, r.Context(), groupId, currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
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
	if ok, err := groups.GroupExists(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
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
	titleExists, err := titles.TitleExists(api.Db, r.Context(), titleID)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	var title titles.Title
	if !titleExists {
		logger.Printf("Title %s not found in main titles collection, adding it", titleID)
		title, err = titles.AddNewTitle(api.Db, api.Provider, r.Context(), titleID)
		if err != nil {
			if code, ok := titles.ErrorMap[err]; ok {
				respondWithError(w, code, err.Error())
				return
			}
			logger.Printf("ERROR: adding new title %s: %v", titleID, err)
			respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
			return
		}
	} else {
		logger.Printf("Title %s found in main titles collection, getting it", titleID)
		title, err = titles.GetTitleById(api.Db, r.Context(), titleID)
		if err != nil {
			logger.Printf("ERROR: getting title %s: %v", titleID, err)
			respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
			return
		}
	}

	err = groups.AddTitleToGroup(api.Db, r.Context(), groupId, titleID, currentUser.Id)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	activity.Record(r.Context(), activity.TitleAdded(groupId, titleID, title.PrimaryTitle))

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

	if ok, err := groups.GroupContainsTitle(api.Db, r.Context(), groupId, req.TitleId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", groupId, req.TitleId))
		return
	}

	title, err := titles.GetTitleById(api.Db, r.Context(), req.TitleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	groupTitle, change, err := groups.UpdateGroupTitleWatched(api.Db, r.Context(), groupId, title, currentUser.Id, req.Watched, req.WatchedAt, req.Season)
	if err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	// The change, not groupTitle: for a season-scoped update the title's own
	// watched flag is a rollup over every season, and the event has to describe
	// the season that was actually changed.
	activity.Record(r.Context(), activity.TitleWatchedChanged(groupId, req.TitleId, title.PrimaryTitle,
		activity.WatchedState{Watched: change.Current.Watched, WatchedAt: change.Current.WatchedAt},
		activity.WatchedState{Watched: change.Previous.Watched, WatchedAt: change.Previous.WatchedAt},
		req.Season))

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

	if ok, err := groups.GroupExists(api.Db, r.Context(), groupId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group with id %s not found", groupId))
		return
	}

	// Read the title before the delete: RemoveTitleFromGroup only removes the
	// group_titles row, but the catalogue title is what carries the name the
	// activity event needs, so it must be captured here rather than after.
	title, err := titles.GetTitleById(api.Db, r.Context(), titleId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	if err := groups.RemoveTitleFromGroup(api.Db, r.Context(), groupId, titleId, currentUser.Id); err != nil {
		if statusCode, ok := groups.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	activity.Record(r.Context(), activity.TitleRemoved(groupId, titleId, title.PrimaryTitle))

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Title %s deleted from group %s", titleId, groupId)})
}
