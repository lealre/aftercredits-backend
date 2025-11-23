package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
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

	if authReq.User == "" {
		respondWithError(w, http.StatusBadRequest, "Field user cannot be null")
		return
	}
	if authReq.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Field password cannot be null")
		return
	}

	// Next steps:
	// 1 - Get user by email or some unique record (username?)
	// 2 - Check passwordHash
	// 3 - Send the token

	// err = auth.CheckPasswordHash(userDB.HashedPassword, userCreds.Password)
	// if err != nil {
	// 	fmt.Printf("Password mismatch for user: %s. Error: %v\n", userCreds.Email, err)
	// 	respondWithError(w, http.StatusUnauthorized, "Credentials are not correct")
	// 	return
	// }

	// token, err := auth.MakeJWT(userDB.ID, cfg.Secret, defaultExpiresAt)
	// if err != nil {
	// 	fmt.Printf("Error creatinf JWT: %s\n", err)
	// 	respondWithError(w, http.StatusInternalServerError, "Something went wrong")
	// 	return
	// }

	respondWithJSON(w, http.StatusOK, auth.LoginResponse{AccessToken: "123"})
}
