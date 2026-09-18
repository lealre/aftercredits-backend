package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/metrics"
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
func NewServer(ctx context.Context, st store.Store, m *metrics.Metrics) (http.Handler, error) {
	provider, err := factory.NewFromEnv()
	if err != nil {
		return nil, err
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set")
	}
	slog.Info("title provider selected", "provider", provider.Name())
	return NewServerWithProvider(ctx, st, provider, secret, m), nil
}

// NewServerWithProvider builds the server with an explicit title provider and
// JWT secret. Tests use this to inject a fixture-backed fake provider (no
// network) and a test secret. ctx bounds the background work it starts — see
// NewServer. m is the metrics registry the chain reports into; it is never nil,
// and callers with no pool pass metrics.New(nil).
func NewServerWithProvider(ctx context.Context, st store.Store, provider titleprovider.Provider, secret string, m *metrics.Metrics) http.Handler {
	mux := http.NewServeMux()

	a := api.NewAPI(st, provider)

	a.Secret = &secret

	mux.HandleFunc("POST /login", a.LoginHandler)

	mux.HandleFunc("GET /users", a.GetUsers)
	mux.HandleFunc("GET /users/me", a.GetUserMe)
	mux.HandleFunc("POST /users/me/password", a.ChangePassword)
	mux.HandleFunc("POST /users/me/logout-all", a.LogoutEverywhere)
	mux.HandleFunc("GET /users/{id}", a.GetUserById)
	mux.HandleFunc("POST /users", a.CreateUser)
	mux.HandleFunc("PATCH /users/{id}", a.UpdateUserInfo)
	mux.HandleFunc("PATCH /users/{id}/active", a.SetUserActive)
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
	// Innermost, directly around the mux: http.ServeMux sets Request.Pattern
	// on the request object it is given, and every WithContext above replaces
	// that object, so this is the only position that can read the pattern.
	handler = metrics.RouteCaptureMiddleware(handler)
	// Bound each handler's context so a slow query cannot pin a pool
	// connection. Exempts GET /activity/stream by path. Sits inside the
	// activity middleware so the post-response event flush is not cancelled
	// by it.
	handler = RequestTimeoutMiddleware(handler)
	if activityFeedEnabled {
		handler = ActivityMiddleware(activity.NewStoreSink(st))(handler)
	}
	handler = AuthMiddleware(*a.Secret, st)(handler)
	// Inside the request-id middleware so it sees every request, including
	// the ones auth rejects before the mux — a 401 rate is worth seeing.
	handler = m.Middleware()(handler)
	handler = RequestIdMiddleware(handler) // wrap LAST → runs FIRST

	return handler
}

// startMetricsListener serves the metrics endpoint on its own listener.
//
// The separation is the point: :8080 sits behind nginx and an auth allowlist,
// while this endpoint has no authentication at all. What keeps it out of reach
// is that container deployments never publish the port — Prometheus scrapes
// over the compose network — so the auth middleware needs no new exception.
//
// That safety is a property of the deployment, not the address. METRICS_ADDR
// defaults to :9090, which binds EVERY interface, so a bare `go run .` on a
// shared network exposes the whole exposition. Set METRICS_ADDR=127.0.0.1:9090
// for non-container runs.
//
// Failing to bind is deliberately not fatal: observability must not stop the
// API from serving.
func startMetricsListener(ctx context.Context, m *metrics.Metrics) {
	if !config.MetricsEnabled() {
		slog.Info("metrics listener disabled", "reason", "METRICS_ENABLED=false")
		return
	}

	addr := config.MetricsAddr()
	// Bind before serving so a failure is reported here, synchronously, rather
	// than from inside a goroutine where it can only be logged after the fact.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("metrics listener cannot bind; continuing without metrics", "err", err, "addr", addr)
		return
	}

	// Only /metrics, not every path. Mounting m.Handler() at the root served the
	// whole exposition to any probe or mistyped scrape path, which contradicted
	// the log line below and gave a scanner the full dump for free.
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())

	srv := &http.Server{
		Handler: mux,
		// Fully bounded, unlike the API server. The API omits these because of
		// the long-lived SSE response; this listener has none, so every timeout
		// is free here and an idle keep-alive connection from anything on the
		// compose network would otherwise accumulate sockets with nothing to
		// reap them.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("metrics listener stopped; the API is unaffected", "err", err, "addr", addr)
		}
	}()
	// Tie the listener's lifetime to the same context that governs the
	// activity LISTEN loop, so it goes away with the server that started it.
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	slog.Info("metrics available", "addr", addr, "path", "/metrics")
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
		slog.Warn("store cannot push activity events; the stream will not deliver live updates", "store", fmt.Sprintf("%T", st))
		return
	}

	go func() {
		if err := listener.ListenActivity(ctx, hub.Publish); err != nil {
			slog.Error("the activity listener stopped", "err", err)
		}
	}()
}

func ListenAndServe(st store.Store, m *metrics.Metrics) error {
	// Cancelled when this function returns — i.e. when the HTTP server has
	// stopped — so the LISTEN loop and its connection go away with it instead
	// of outliving the thing they were started for.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, err := NewServer(ctx, st, m)
	if err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}

	startMetricsListener(ctx, m)

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
		// Cap the header size a client can send.
		MaxHeaderBytes: 1 << 16,
		// WriteTimeout is deliberately NOT set. It is an absolute deadline on
		// the whole response, and the activity feed streams Server-Sent Events
		// over a connection that stays open indefinitely by design — any value
		// here would sever every live feed on a timer. ReadTimeout is likewise
		// left off: it would cap the same long-lived requests, and the request
		// body is already bounded by MaxBytesReader in RequestIdMiddleware and
		// each handler's context by RequestTimeoutMiddleware.
	}
	slog.Info("server running", "addr", ":8080")
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error while starting server: %v", err)
	}
	slog.Info("server listening", "port", 8080)
	return nil
}
