package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/activity"
)

func (api *API) GetActivityFeed(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	limit := generics.StringToInt(r.URL.Query().Get("limit"))

	var before *int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "before must be a number")
			return
		}
		before = &parsed
	}

	feed, err := activity.GetFeed(api.Db, r.Context(), currentUser.Id, before, limit)
	if err != nil {
		if code, ok := activity.ErrorMap[err]; ok {
			respondWithError(w, code, err.Error())
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	respondWithJSON(w, http.StatusOK, feed)
}

func (api *API) GetActivityUnreadCount(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	count, err := activity.GetUnreadCount(api.Db, r.Context(), currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	respondWithJSON(w, http.StatusOK, count)
}

func (api *API) MarkActivityRead(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	var req activity.MarkReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := activity.MarkRead(api.Db, r.Context(), currentUser.Id, req.Seq); err != nil {
		if code, ok := activity.ErrorMap[err]; ok {
			respondWithError(w, code, err.Error())
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
