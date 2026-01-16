package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/users"
)

func (api *API) GetUsers(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	user := auth.GetUserFromContext(r.Context())

	if user.Role != mongodb.RoleAdmin {
		respondWithForbidden(w)
		return
	}

	allUsers, err := users.GetAllUsers(api.Db, r.Context())
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database lookup failed")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: allUsers})
}

func (api *API) GetUserById(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currnetUser := auth.GetUserFromContext(r.Context())

	userId := r.PathValue("id")
	if userId == "" {
		respondWithError(w, http.StatusBadRequest, "User id is required")
		return
	}

	if currnetUser.Id != userId {
		respondWithForbidden(w)
		return
	}

	user, err := users.GetUserById(api.Db, r.Context(), userId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database lookup failed")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func (api *API) GetUserMe(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	user, err := users.GetUserById(api.Db, r.Context(), currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database lookup failed")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func (api *API) UpdateUserInfo(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currnetUser := auth.GetUserFromContext(r.Context())

	userId := r.PathValue("id")
	if userId == "" {
		respondWithError(w, http.StatusBadRequest, "User id is required")
		return
	}

	if currnetUser.Id != userId {
		respondWithForbidden(w)
		return
	}

	var req users.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	user, err := users.UpdateUserInfo(api.Db, r.Context(), userId, req)
	if err != nil {
		// Check custom erros from fileds validations
		if statusCode, ok := users.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database lookup failed")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

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
		// Check custom erros from fileds validations
		if statusCode, ok := users.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
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
	user := auth.GetUserFromContext(r.Context())

	userId := r.PathValue("id")
	if userId == "" {
		respondWithError(w, http.StatusBadRequest, "User id is required")
		return
	}

	if user.Id != userId {
		respondWithForbidden(w)
		return
	}

	if err := users.DeleteUserById(api.Db, r.Context(), userId); err != nil {
		if err == mongodb.ErrRecordNotFound {
			logger.Printf("WARNING: Attempted deletion of own user ID failed because user was not found. ERROR: %v", err)
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", userId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting user")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("User with id %s deleted successfully", userId)})
}
