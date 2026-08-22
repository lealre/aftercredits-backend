package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/config"
	activityservice "github.com/lealre/movies-backend/internal/services/activity"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

// NewServer builds the production server, selecting the title provider from env
// and requiring the JWT signing secret (JWT_SECRET) to be set.
//
// ctx bounds the background work the server starts — today the activity
// LISTEN loop. Cancelling it stops that loop and closes its database
// connection; it is not the context of any request.
func NewServer(ctx context.Context, st store.Store) (http.Handler, error) {
	provider, err := factory.NewFromEnv()
	if err != nil {
		return nil, err
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set")
	}
	log.Printf("Using title provider: %s", provider.Name())
	return NewServerWithProvider(ctx, st, provider, secret), nil
}

// NewServerWithProvider builds the server with an explicit title provider and
// JWT secret. Tests use this to inject a fixture-backed fake provider (no
// network) and a test secret. ctx bounds the background work it starts — see
// NewServer.
func NewServerWithProvider(ctx context.Context, st store.Store, provider titleprovider.Provider, secret string) http.Handler {
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
	// One title's group-scoped detail, same shape as one element of the list
	// above. An ordinary group-titles read: the activity feed uses it to
	// deep-link a row to that title's modal, but nothing about it is
	// feed-specific, so ACTIVITY_FEED_ENABLED does not gate it.
	mux.HandleFunc("GET /groups/{groupId}/titles/{titleId}", a.GetTitleFromGroup)
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
		// Read state is per event, so the two things a client can do have
		// separate routes rather than one route whose body decides: marking one
		// row read names that row in its path, and clearing the badge says so.
		// The old POST /activity/read (a watermark seq in the body) is gone
		// rather than reinterpreted — it never shipped enabled, and silently
		// changing what a body meant would be worse than removing it.
		mux.HandleFunc("POST /activity/events/{id}/read", a.MarkActivityEventRead)
		mux.HandleFunc("POST /activity/read-all", a.MarkAllActivityRead)

		// Everything live is built here and nowhere else: with the flag off
		// there is no hub, no ticket store, no listener goroutine, no
		// dedicated LISTEN connection, and neither route exists to be called.
		hub := activity.NewHub()
		a.Stream = activityservice.NewStreamer(hub, activity.NewTicketStore())

		mux.HandleFunc("POST /activity/stream-ticket", a.IssueActivityStreamTicket)
		mux.HandleFunc("GET /activity/stream", a.StreamActivity)

		startActivityListener(ctx, st, hub)
	}

	var handler http.Handler = mux
	if activityFeedEnabled {
		handler = ActivityMiddleware(activity.NewStoreSink(st))(handler)
	}
	handler = AuthMiddleware(*a.Secret, st)(handler)
	handler = RequestIdMiddleware(handler) // wrap LAST → runs FIRST

	return handler
}

// startActivityListener starts the one LISTEN loop that feeds hub, if this
// store can push at all. It is only ever called with the feature on.
//
// The loop runs until ctx is cancelled, at which point it closes its dedicated
// database connection and returns; it reconnects with backoff on its own for
// anything short of that, so an error coming back out of it means it has given
// up for good.
func startActivityListener(ctx context.Context, st store.Store, hub *activity.Hub) {
	listener, ok := st.(store.ActivityListener)
	if !ok {
		// Not a failure worth refusing to boot over: the feed and the stream
		// both still work, the stream just stays silent until the client's
		// next snapshot. Said once, loudly, rather than swallowed.
		log.Printf("WARN: %T cannot push activity events; the stream will not deliver live updates", st)
		return
	}

	go func() {
		if err := listener.ListenActivity(ctx, hub.Publish); err != nil {
			log.Printf("ERROR: the activity listener stopped: %v", err)
		}
	}()
}

func ListenAndServe(st store.Store) error {
	// Cancelled when this function returns — i.e. when the HTTP server has
	// stopped — so the LISTEN loop and its connection go away with it instead
	// of outliving the thing they were started for.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, err := NewServer(ctx, st)
	if err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
		// A header-read deadline, so a client cannot hold a connection open by
		// dribbling out request headers forever (Slowloris). Ten seconds is far
		// more than any real client needs to send them.
		ReadHeaderTimeout: 10 * time.Second,
		// Idle keep-alive connections are reaped, which costs nothing and
		// bounds the number of sockets a stalled client can accumulate.
		IdleTimeout: 120 * time.Second,
		// WriteTimeout is deliberately NOT set. It is an absolute deadline on
		// the whole response, and the activity feed streams Server-Sent Events
		// over a connection that stays open indefinitely by design — any value
		// here would sever every live feed on a timer. ReadTimeout is likewise
		// left off: it would cap the same long-lived requests, and there are no
		// request bodies large enough to need it.
	}
	log.Println("Server running on :8080")
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error while starting server: %v", err)
	}
	log.Println("Server started listening on port 8080")
	return nil
}
