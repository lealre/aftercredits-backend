package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
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

	if authReq.Username == "" && authReq.Email == "" {
		respondWithError(w, http.StatusBadRequest, "One of the fields Username or Email cannot be null")
		return
	}
	if authReq.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Field password cannot be null")
		return
	}

	userDb, err := api.Db.GetUserByUsernameOrEmail(r.Context(), authReq.Username, authReq.Email)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error looking for User")
		return
	}

	err = auth.CheckPasswordHash(userDb.PasswordHash, authReq.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Credentials are not correct")
		return
	}

	token, err := auth.MakeJWT(userDb.Id, *api.Secret, defaultExpiresAt)
	if err != nil {
		logger.Printf("EROOR: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	respondWithJSON(w, http.StatusOK, auth.LoginResponse{AccessToken: token})
}
