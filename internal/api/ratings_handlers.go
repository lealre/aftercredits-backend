package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
)

func (api *API) GetRatingById(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	rating, err := ratings.GetRatingById(api.Db, r.Context(), ratingId, currentuser.Id)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Rating with id %s not found", ratingId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database error while getting rating")
		return
	}

	respondWithJSON(w, http.StatusOK, rating)
}

func (api *API) AddRating(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	var req ratings.NewRating
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	if req.Note < 0 || req.Note > 10 {
		respondWithError(w, http.StatusBadRequest, "Note must be between 0 and 10")
		return
	}

	if ok, err := api.Db.GroupContainsTitle(r.Context(), req.GroupId, req.TitleId, currentuser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", req.GroupId, req.TitleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	newRating, err := ratings.AddRating(api.Db, r.Context(), req, currentuser.Id)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusCreated, newRating)
}

func (api *API) UpdateRating(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	var updateReq ratings.UpdateRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if updateReq.Note < 0 || updateReq.Note > 10 {
		respondWithError(w, http.StatusBadRequest, "Note must be between 0 and 10")
		return
	}

	updatedRating, err := ratings.UpdateRating(api.Db, r.Context(), ratingId, currentuser.Id, updateReq)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update rating")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedRating)
}
