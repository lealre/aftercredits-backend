package server

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

// NewServer builds the production server, selecting the title provider from env
// and requiring the JWT signing secret (JWT_SECRET) to be set.
func NewServer(st store.Store) (http.Handler, error) {
	provider, err := factory.NewFromEnv()
	if err != nil {
		return nil, err
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set")
	}
	log.Printf("Using title provider: %s", provider.Name())
	return NewServerWithProvider(st, provider, secret), nil
}

// NewServerWithProvider builds the server with an explicit title provider and
// JWT secret. Tests use this to inject a fixture-backed fake provider (no
// network) and a test secret.
func NewServerWithProvider(st store.Store, provider titleprovider.Provider, secret string) http.Handler {
	mux := http.NewServeMux()

	a := api.NewAPI(st, provider)

	a.Secret = &secret

	mux.HandleFunc("POST /login", a.LoginHandler)

	mux.HandleFunc("GET /users", a.GetUsers)
	mux.HandleFunc("GET /users/me", a.GetUserMe)
	mux.HandleFunc("GET /users/{id}", a.GetUserById)
	mux.HandleFunc("POST /users", a.CreateUser)
	mux.HandleFunc("PATCH /users/{id}", a.UpdateUserInfo)
	mux.HandleFunc("DELETE /users/{id}", a.DeleteUserById)

	mux.HandleFunc("POST /groups", a.CreateGroup)
	mux.HandleFunc("GET /groups/{id}", a.GetGroupById)
	mux.HandleFunc("PATCH /groups/{id}", a.UpdateGroup)
	mux.HandleFunc("DELETE /groups/{id}", a.DeleteGroup)
	mux.HandleFunc("DELETE /groups/{id}/users/{userId}", a.RemoveUserFromGroup)
	// Group - Users
	mux.HandleFunc("GET /groups/{id}/users", a.GetUsersFromGroup)
	mux.HandleFunc("POST /groups/{id}/users", a.AddUserToGroup)
	// Group - Titles
	mux.HandleFunc("GET /groups/{id}/titles", a.GetTitlesFromGroup)
	mux.HandleFunc("POST /groups/titles", a.AddTitleToGroup)
	mux.HandleFunc("PATCH /groups/{id}/titles", a.UpdateGroupTitleWatched)
	mux.HandleFunc("DELETE /groups/{groupId}/titles/{titleId}", a.DeleteTitleFromGroup)
	// Group - Comments
	mux.HandleFunc("GET /groups/{groupId}/titles/{titleId}/comments", a.GetCommentsByTitleIDFromGroup)
	mux.HandleFunc("PATCH /groups/{groupId}/titles/{titleId}/comments/{commentId}", a.UpdateComment)
	mux.HandleFunc("DELETE /groups/{groupId}/titles/{titleId}/comments/{commentId}", a.DeleteComment)
	mux.HandleFunc("DELETE /groups/{groupId}/titles/{titleId}/comments/{commentId}/seasons/{season}", a.DeleteCommentSeason)

	mux.HandleFunc("GET /titles", a.GetTitles)
	mux.HandleFunc("GET /titles/search", a.SearchTitles)
	mux.HandleFunc("GET /titles/{id}/episodes", a.GetTitleEpisodes)
	mux.HandleFunc("POST /titles", a.AddTitle)
	mux.HandleFunc("DELETE /titles/{id}", a.DeleteTitle)

	mux.HandleFunc("GET /ratings/{id}", a.GetRatingById)
	mux.HandleFunc("POST /ratings", a.AddRating)
	mux.HandleFunc("PATCH /ratings/{id}", a.UpdateRating)
	mux.HandleFunc("DELETE /ratings/{id}", a.DeleteRating)
	mux.HandleFunc("DELETE /ratings/{id}/seasons/{season}", a.DeleteRatingSeason)

	mux.HandleFunc("POST /comments", a.AddComment)

	// Read once so the middleware below and the route registration just above it
	// agree on the same on/off decision for this server instance — two
	// independent config.ActivityFeedEnabled() calls could disagree if the
	// environment changed mid-process, leaving events recorded with no way to
	// read them.
	activityFeedEnabled := config.ActivityFeedEnabled()

	if activityFeedEnabled {
		mux.HandleFunc("GET /activity", a.GetActivityFeed)
		mux.HandleFunc("GET /activity/unread-count", a.GetActivityUnreadCount)
		mux.HandleFunc("POST /activity/read", a.MarkActivityRead)
	}

	var handler http.Handler = mux
	if activityFeedEnabled {
		handler = ActivityMiddleware(activity.NewStoreSink(st))(handler)
	}
	handler = AuthMiddleware(*a.Secret, st)(handler)
	handler = RequestIdMiddleware(handler) // wrap LAST → runs FIRST

	return handler
}

func ListenAndServe(st store.Store) error {
	handler, err := NewServer(st)
	if err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	log.Println("Server running on :8080")
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error while starting server: %v", err)
	}
	log.Println("Server started listening on port 8080")
	return nil
}
