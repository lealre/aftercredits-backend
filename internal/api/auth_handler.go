package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/users"
)

const defaultExpiresAt = time.Second * 60 * 60

func (api *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var authReq auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if strings.TrimSpace(authReq.Username) == "" && strings.TrimSpace(authReq.Email) == "" {
		respondWithError(w, http.StatusBadRequest, "One of the fields Username or Email cannot be null")
		return
	}
	if strings.TrimSpace(authReq.Password) == "" {
		respondWithError(w, http.StatusBadRequest, "Field password cannot be null")
		return
	}

	userDb, err := users.GetUserDbByUsernameOrEmail(api.Db, r.Context(), authReq.Username, authReq.Email)
	if err != nil {
		if statusCode, ok := users.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while looking for User")
		return
	}

	err = auth.CheckPasswordHash(userDb.PasswordHash, authReq.Password)
	if err != nil {
		if statusCode, ok := auth.ErrorsMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	token, err := auth.MakeJWT(userDb.Id, *api.Secret, defaultExpiresAt)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	userLoginResponse, err := users.BuildLoginResponse(api.Db, r.Context(), userDb, token)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, userLoginResponse)
}
