package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
	"go.mongodb.org/mongo-driver/mongo"
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Home",
	})
}

func GetTitlessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get all titles from MongoDB
	cursor, err := titles.GetAllTitles(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch movies from database")
		return
	}
	defer cursor.Close(ctx)

	var allMovies []titles.Title

	// Iterate through the cursor and map each title to a movie
	for cursor.Next(ctx) {
		var title imdb.Title
		if err := cursor.Decode(&title); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to decode movie data")
			return
		}

		movie := titles.MapImdbTitleToDbTitle(title)
		allMovies = append(allMovies, movie)
	}

	// Check for cursor errors
	if err := cursor.Err(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database cursor error")
		return
	}

	// Return the list of movies
	respondWithJSON(w, http.StatusOK, titles.AllMoviesResponse{Movies: allMovies})
}

func AddTitleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req titles.AddMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.URL == "" {
		respondWithError(w, http.StatusBadRequest, "url is required")
		return
	}

	// Accept URLs like https://www.imdb.com/title/tt8009428/ and extract the ID (tt...)
	re := regexp.MustCompile(`^https?://(?:www\.)?imdb\.com/title/(tt[0-9]+)/?`)
	m := re.FindStringSubmatch(req.URL)
	if len(m) != 2 {
		respondWithError(w, http.StatusBadRequest, "invalid IMDb title URL")
		return
	}
	titleID := m[1]

	ctx := context.Background()

	// First, check if the document already exists
	if _, err := titles.GetTitleByID(ctx, titleID); err == nil {
		respondWithError(w, http.StatusBadRequest, "title already added")
		return
	} else if err != mongodb.ErrRecordNotFound {
		log.Printf("Error getting title by ID: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}

	// Not found; fetch from IMDb API and store
	body, err := imdb.FetchMovie(titleID)
	if err != nil {
		respondWithError(w, http.StatusBadGateway, "failed to fetch title from IMDb API")
		return
	}

	// Keep a concrete copy for response mapping
	var title imdb.Title
	if err := json.Unmarshal(body, &title); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to parse IMDb API response")
		return
	}

	// Store in MongoDB using generic map to preserve structure and _id
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to parse payload for storage")
		return
	}
	// Ensure _id is set for MongoDB primary key
	if idVal, ok := doc["id"].(string); ok && idVal != "" {
		doc["_id"] = idVal
	}

	if err := titles.AddTitle(ctx, doc); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusInternalServerError, "failed to store title in database")
			return
		}
		// If duplicate, try to read back the stored document
		if stored, gerr := titles.GetTitleByID(ctx, titleID); gerr == nil && stored != nil {
			raw, _ := json.Marshal(stored)
			_ = json.Unmarshal(raw, &title)
		}
	}

	// Map to API movie type and respond
	movie := titles.MapImdbTitleToDbTitle(title)
	respondWithJSON(w, http.StatusCreated, movie)
}

func GetAllRatingsHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusAccepted, "Not Implemented")
}

func GetRatingByIdHandler(w http.ResponseWriter, r *http.Request) {
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
	var req ratings.Rating
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	ctx := context.Background()
	if ok, err := users.CheckIfUserExist(ctx, req.UserId); err != nil {
		log.Println("Error checking user in database:", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", req.UserId))
		return
	}

	if ok, err := titles.ChecKIfTitleExist(ctx, req.TitleId); err != nil {
		log.Println("Error checking title in database:", err)
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

func GetTitleRatingsHandler(w http.ResponseWriter, r *http.Request) {
	titleId := r.PathValue("id")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

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

func GetUsers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	cursor, err := users.GetAllUsers(ctx)
	if err != nil {
		log.Printf("Error getting all users: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}
	defer cursor.Close(ctx)

	var allUsers []users.User
	if err := cursor.All(ctx, &allUsers); err != nil {
		log.Printf("Error decoding users: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to decode users")
		return
	}

	if len(allUsers) == 0 {
		respondWithError(w, http.StatusNotFound, "No users found")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: allUsers})
}

func UpdateRatingHandler(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("Error updating rating: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update rating")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Rating updated successfully"})
}
