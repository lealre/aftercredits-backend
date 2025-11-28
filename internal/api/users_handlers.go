package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
  - handle invalid password
  - validate email format
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

	if strings.TrimSpace(req.Username) == "" && strings.TrimSpace(req.Email) == "" {
		respondWithError(w, http.StatusBadRequest, "Username or Email fields are required.")
		return
	}
	if req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Password field is required.")
		return
	}

	user, err := users.AddUser(api.Db, r.Context(), req)
	if err != nil {
		if errors.Is(err, users.ErrUsernameAlreadyExists) {
			respondWithError(w, http.StatusBadRequest, "Username already exists")
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user")
		return
	}

	respondWithJSON(w, http.StatusCreated, user)
}

func (api *API) DeleteUserById(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	userId := r.PathValue("id")
	if userId == "" {
		respondWithError(w, http.StatusBadRequest, "User id is required")
		return
	}

	if ok, err := api.Db.UserExists(r.Context(), userId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	} else if !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", userId))
		return
	}

	if err := users.DeleteUserById(api.Db, r.Context(), userId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting user")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("User with id %s deleted successfully", userId)})
}
