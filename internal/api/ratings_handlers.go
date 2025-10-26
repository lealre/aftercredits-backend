package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetRatingById(w http.ResponseWriter, r *http.Request) {
	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "rating id is required")
		return
	}

	// Get rating by ID
	ctx := context.Background()
	rating, err := ratings.GetRatingById(ctx, ratingId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Rating with id %s not found", ratingId))
			return
		}
		respondWithError(w, http.StatusInternalServerError, "database error while getting rating")
		return
	}

	respondWithJSON(w, http.StatusOK, rating)
}

func AddRating(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var req ratings.Rating
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	ctx := context.Background()
	if ok, err := users.CheckIfUserExist(ctx, req.UserId); err != nil {
		logger.Println("Error checking user in database:", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", req.UserId))
		return
	}

	if ok, err := titles.ChecKIfTitleExist(ctx, req.TitleId); err != nil {
		logger.Println("Error checking title in database:", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", req.TitleId))
		return
	}

	if err := ratings.AddRating(ctx, req); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusBadRequest, "Rating already exists for this user and title")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while adding rating")
		return
	}

	respondWithJSON(w, http.StatusCreated, req)
}

func GetTitleRatings(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}
	logger.Printf("Getting ratings for title id %s", titleId)

	ctx := context.Background()
	if ok, err := titles.ChecKIfTitleExist(ctx, titleId); err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error while checking title")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", titleId))
		return
	}

	// Get all ratings for this title
	titleRatings, err := ratings.GetRatingsByTitleId(ctx, titleId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error while getting ratings")
		return
	}

	if len(titleRatings) == 0 {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("No ratings found for title with id %s", titleId))
		return
	}

	allRatings := ratings.AllRatingsFromMovie{Ratings: titleRatings}
	respondWithJSON(w, http.StatusOK, allRatings)
}

func UpdateRating(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	ratingId := r.PathValue("id")
	if ratingId == "" {
		respondWithError(w, http.StatusBadRequest, "Rating id is required")
		return
	}

	// Parse request body
	var updateReq ratings.UpdateRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate the update request
	if updateReq.Note < 0 || updateReq.Note > 10 {
		respondWithError(w, http.StatusBadRequest, "Note must be between 1 and 10")
		return
	}

	ctx := context.Background()

	// Update the rating
	if err := ratings.UpdateRating(ctx, ratingId, updateReq); err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Rating with id %s not found", ratingId))
			return
		}
		logger.Printf("Error updating rating: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update rating")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Rating updated successfully"})
}
