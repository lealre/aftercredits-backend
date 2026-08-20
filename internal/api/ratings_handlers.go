package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/groups"
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
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
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

	// Establishes both that the group is real and that the caller is a member of
	// it, so req.GroupId is safe to persist on the rating below.
	if ok, err := groups.GroupContainsTitle(api.Db, r.Context(), req.GroupId, req.TitleId, currentuser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", req.GroupId, req.TitleId))
		return
	}

	// AddRating already has to load the title to route movie vs TV series
	// logic, so it hands it back here rather than the handler looking it up
	// again by the same id.
	newRating, title, err := ratings.AddRating(api.Db, r.Context(), req, currentuser.Id)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	activity.Record(r.Context(), activity.RatingAdded(req.GroupId, req.TitleId, title.PrimaryTitle, req.Note, req.Season))

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

	// UpdateRating hands back the rating as it stood immediately before the
	// update, from the same read that drove the write — not a second,
	// independently-timed snapshot a concurrent update could race with.
	updatedRating, previousRating, err := ratings.UpdateRating(api.Db, r.Context(), ratingId, currentuser.Id, updateReq)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update rating")
		return
	}

	// The title itself carries no race risk (title rows are effectively
	// immutable), so a plain lookup by id is fine here.
	title, err := titles.GetTitleById(api.Db, r.Context(), previousRating.TitleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	activity.Record(r.Context(), activity.RatingUpdated(previousRating.GroupId, previousRating.TitleId, title.PrimaryTitle, updateReq.Note, previousRating.Note, updateReq.Season))

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

	// Read the rating (and its title) before it is gone: DeleteRating only
	// reports how many rows were removed, not what they were.
	rating, err := ratings.GetRatingById(api.Db, r.Context(), ratingId, currentuser.Id)
	if err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting rating")
		return
	}

	title, err := titles.GetTitleById(api.Db, r.Context(), rating.TitleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	if _, err := ratings.DeleteRating(api.Db, r.Context(), ratingId, currentuser.Id); err != nil {
		if statusCode, ok := ratings.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting rating")
		return
	}

	activity.Record(r.Context(), activity.RatingDeleted(rating.GroupId, rating.TitleId, title.PrimaryTitle, rating.Note))

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

	// seasonStr is guaranteed to parse here: DeleteRatingSeason above performs
	// the identical conversion and would have failed already if it did not.
	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		logger.Printf("ERROR: unexpected invalid season %q after a successful delete: %v", seasonStr, err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	// rating was fetched before the delete above, so its SeasonsRatings still
	// holds the value that is being removed.
	var previousNote float64
	if rating.SeasonsRatings != nil {
		if seasonRating, ok := (*rating.SeasonsRatings)[seasonStr]; ok {
			previousNote = seasonRating.Rating
		}
	}

	activity.Record(r.Context(), activity.RatingSeasonDeleted(rating.GroupId, rating.TitleId, title.PrimaryTitle, season, previousNote))

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Season %s from rating %s deleted successfully", seasonStr, ratingId)})
}
