package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"go.mongodb.org/mongo-driver/mongo"
)

func (api *API) GetRatingById(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	ctx := context.Background()
	rating, err := ratings.GetRatingById(api.Db, ctx, ratingId, currentuser.Id)
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

	var req ratings.Rating
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	if currentuser.Id != req.UserId {
		respondWithForbidden(w)
		return
	}

	if ok, err := api.Db.TitleExists(r.Context(), req.TitleId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", req.TitleId))
		return
	}

	if err := ratings.AddRating(api.Db, r.Context(), req); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusBadRequest, "Rating already exists for this user and title")
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while adding rating")
		return
	}

	respondWithJSON(w, http.StatusCreated, req)
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

	if err := ratings.UpdateRating(api.Db, r.Context(), ratingId, currentuser.Id, updateReq); err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Rating with id %s not found", ratingId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update rating")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Rating updated successfully"})
}

func (api *API) GetRatingsBatchByTitleIDs(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentuser := auth.GetUserFromContext(r.Context())

	var req ratings.GetRatingsBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	titlesRatingsMap, err := ratings.GetRatingsBatch(api.Db, r.Context(), req.Titles, currentuser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error getting ratings from titles list")
		return
	}

	respondWithJSON(w, http.StatusOK, titlesRatingsMap)

}
