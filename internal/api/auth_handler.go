package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/users"
)

const defaultExpiresAt = time.Second * 60 * 60

// invalidCredentialsMessage is returned, byte-identical, for every failed login
// — unknown account, wrong password, or deactivated account — so the response
// leaks nothing about which one it was.
const invalidCredentialsMessage = "Invalid username or password"

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
		// Do NOT distinguish "no such account" from "wrong password". Returning
		// the service's 404 here (as this used to) turns login into a
		// username-existence oracle, and skipping bcrypt on the not-found path
		// leaks the same fact through timing. Burn an equivalent bcrypt on the
		// not-found path and return the identical 401 the wrong-password branch
		// returns below.
		if errors.Is(err, users.ErrUserNotFound) {
			auth.FakePasswordCheck(authReq.Password)
			respondWithError(w, http.StatusUnauthorized, invalidCredentialsMessage)
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while looking for User")
		return
	}

	if err := auth.CheckPasswordHash(userDb.PasswordHash, authReq.Password); err != nil {
		respondWithError(w, http.StatusUnauthorized, invalidCredentialsMessage)
		return
	}

	// A deactivated account must not be able to log in. Return the same generic
	// 401 rather than a distinct "account disabled", so the response reveals
	// nothing an attacker can act on.
	if !userDb.IsActive {
		respondWithError(w, http.StatusUnauthorized, invalidCredentialsMessage)
		return
	}

	token, err := auth.MakeJWT(userDb.Id, userDb.TokenVersion, *api.Secret, defaultExpiresAt)
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
