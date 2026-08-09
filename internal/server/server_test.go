package server_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/server"
	"github.com/lealre/movies-backend/internal/store"
)

// NewServer must refuse to start when JWT_SECRET is unset (no insecure fallback).
func TestNewServer_RequiresJWTSecret(t *testing.T) {
	// imdbapi provider needs no API key, so the factory succeeds and we reach
	// the JWT_SECRET check without needing a DB connection.
	t.Setenv("TITLE_PROVIDER", "imdbapi")
	t.Setenv("JWT_SECRET", "")

	_, err := server.NewServer(nil)
	if err == nil {
		t.Fatal("expected NewServer to error when JWT_SECRET is unset")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected a JWT_SECRET error, got: %v", err)
	}
}

// stubUserStore satisfies store.Store by embedding the interface, so only the
// one method the auth middleware calls needs an implementation. The embedded
// interface is nil: any other call would panic, which is the point — it proves
// the middleware touches nothing else.
type stubUserStore struct {
	store.Store
	user models.User
	err  error
}

func (s stubUserStore) GetUserById(context.Context, string) (models.User, error) {
	return s.user, s.err
}

// TestAuthMiddleware_UserLookup pins how the middleware treats the outcome of
// the user lookup. It used to fold every case into one condition:
//
//	if errors.Is(err, store.ErrRecordNotFound) || !userDb.IsActive {
//
// On any other error userDb is the zero models.User, so IsActive is false and a
// transient database failure was reported to the client as 401 "Invalid or
// inactive user" — silently logging everyone out — with err never logged.
// A store failure must be a logged 500; only a genuinely missing or deactivated
// user is a 401.
func TestAuthMiddleware_UserLookup(t *testing.T) {
	const secret = "middleware-test-secret"
	const userId = "11111111-1111-1111-1111-111111111111"

	token, err := auth.MakeJWT(userId, secret, time.Hour)
	require.NoError(t, err, "failed to mint a test token")

	activeUser := models.User{Id: userId, Username: "active", IsActive: true}

	// call runs one authenticated request through the middleware against the
	// given store, returning the response, whether the wrapped handler ran, and
	// everything the request logger recorded.
	call := func(t *testing.T, st store.Store) (*httptest.ResponseRecorder, bool, string) {
		t.Helper()

		reached := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		})

		var logged bytes.Buffer
		req := httptest.NewRequest(http.MethodGet, "/groups/some-group", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req = req.WithContext(logx.WithLogger(req.Context(), log.New(&logged, "", 0)))

		recorder := httptest.NewRecorder()
		server.AuthMiddleware(secret, st)(next).ServeHTTP(recorder, req)

		return recorder, reached, logged.String()
	}

	t.Run("a store failure is logged and answered with 500", func(t *testing.T) {
		storeErr := errors.New("dial tcp 127.0.0.1:5432: connect: connection refused")

		resp, reached, logged := call(t, stubUserStore{err: storeErr})

		require.Equal(t, http.StatusInternalServerError, resp.Code,
			"an unexpected store failure must be a 500, not a 401 that logs the user out")
		require.NotContains(t, resp.Body.String(), "Invalid or inactive user",
			"a store failure must not be reported to the client as an invalid user")
		require.Contains(t, logged, storeErr.Error(),
			"the store failure must be logged, not swallowed")
		require.False(t, reached, "the wrapped handler must not run when the user lookup fails")
	})

	t.Run("an unknown user still gets 401", func(t *testing.T) {
		resp, reached, _ := call(t, stubUserStore{err: store.ErrRecordNotFound})

		require.Equal(t, http.StatusUnauthorized, resp.Code, "a missing user is still a 401")
		require.Contains(t, resp.Body.String(), "Invalid or inactive user",
			"the 401 body for a missing user must be unchanged")
		require.False(t, reached, "the wrapped handler must not run for an unknown user")
	})

	t.Run("a deactivated user still gets 401", func(t *testing.T) {
		inactive := activeUser
		inactive.IsActive = false

		resp, reached, _ := call(t, stubUserStore{user: inactive})

		require.Equal(t, http.StatusUnauthorized, resp.Code, "an inactive user is still a 401")
		require.Contains(t, resp.Body.String(), "Invalid or inactive user",
			"the 401 body for an inactive user must be unchanged")
		require.False(t, reached, "the wrapped handler must not run for an inactive user")
	})

	t.Run("an active user reaches the handler with the user in context", func(t *testing.T) {
		var seen *models.User
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = auth.GetUserFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/groups/some-group", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()

		server.AuthMiddleware(secret, stubUserStore{user: activeUser})(next).ServeHTTP(recorder, req)

		require.Equal(t, http.StatusOK, recorder.Code, "an active user must be let through")
		require.NotNil(t, seen, "the authenticated user must be put in the request context")
		require.Equal(t, userId, seen.Id, "the context must carry the looked-up user")
	})
}
