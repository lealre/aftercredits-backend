package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/lealre/movies-backend/internal/imdb"
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

	var allMovies []titles.Movie

	// Iterate through the cursor and map each title to a movie
	for cursor.Next(ctx) {
		var title imdb.Title
		if err := cursor.Decode(&title); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to decode movie data")
			return
		}

		movie := titles.MapTitleToMovie(title)
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
	} else if err != mongo.ErrNoDocuments {
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
	movie := titles.MapTitleToMovie(title)
	respondWithJSON(w, http.StatusCreated, movie)
}

func GetAllRatings(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusAccepted, "Not Implemented")
}

func GetRatingById(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusAccepted, "Not Implemented")
}

func AddRating(w http.ResponseWriter, r *http.Request) {
	var req ratings.Rating
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading request Body")
		return
	}

	// Check if user exists in Database
	ctx := context.Background()
	if ok, err := users.CheckIfUserExist(ctx, req.UserId); err != nil {
		respondWithError(w, http.StatusInternalServerError, "database error while checking user")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, "user not found")
		return
	}
	// Check if movie exists in database
	// req.
	respondWithJSON(w, http.StatusAccepted, "Not Implemented")
}
