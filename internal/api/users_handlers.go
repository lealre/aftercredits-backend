package api

import (
	"context"
	"net/http"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/users"
)

func GetUsers(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	ctx := context.Background()
	cursor, err := users.GetAllUsers(ctx)
	if err != nil {
		logger.Printf("Error getting all users: %v", err)
		respondWithError(w, http.StatusInternalServerError, "database lookup failed")
		return
	}
	defer cursor.Close(ctx)

	var allUsers []users.User
	if err := cursor.All(ctx, &allUsers); err != nil {
		logger.Printf("Error decoding users: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to decode users")
		return
	}

	if len(allUsers) == 0 {
		respondWithError(w, http.StatusNotFound, "No users found")
		return
	}

	respondWithJSON(w, http.StatusOK, users.AllUsersResponse{Users: allUsers})
}
