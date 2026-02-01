package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
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

func (api *API) DeleteRating(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	_, err := ratings.DeleteRating(api.Db, r.Context(), ratingId, currentuser.Id)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting rating")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Rating with id %s deleted successfully", ratingId)})
}

func (api *API) DeleteRatingSeason(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	seasonStr := r.PathValue("season")
	if seasonStr == "" {
		respondWithError(w, http.StatusBadRequest, "Season number is required")
		return
	}

	// Get the rating to find the titleId
	rating, err := ratings.GetRatingById(api.Db, r.Context(), ratingId, currentuser.Id)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	// Get the title to validate season
	title, err := titles.GetTitleById(api.Db, r.Context(), rating.TitleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	if err := ratings.DeleteRatingSeason(api.Db, r.Context(), ratingId, currentuser.Id, seasonStr, title); err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting season rating")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Season %s from rating %s deleted successfully", seasonStr, ratingId)})
}
