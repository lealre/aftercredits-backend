package api

import (
	"encoding/json"
	"net/http"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/users"
)

func (api *API) GetUsers(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	allUsers, err := users.GetAllUsers(api.Db, r.Context())
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database lookup failed")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: allUsers})
}

/*
TODO:
  - handle duplicated username
  - handle invalid password
  - Validate email format
  - handle duplicated email
*/
func (api *API) CreateUser(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var req users.NewUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if req.Name == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and Password fields are required.")
		return
	}

	user, err := users.AddUser(api.Db, r.Context(), req)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user")
		return
	}

	respondWithJSON(w, http.StatusCreated, user)
}
