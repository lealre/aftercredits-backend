package api

import (
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
