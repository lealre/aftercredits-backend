package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/server"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// These tests are about the wiring, not the handlers: that the flag gates both
// routes AND the listener, that the stream route survives AuthMiddleware while
// the ticket route does not, and that an event injected where Postgres would
// inject it comes out of a real HTTP connection through the whole middleware
// chain. The handler's own behaviour is pinned in internal/api.

const (
	streamWireTimeout = 5 * time.Second
	streamWireSecret  = "stream-wiring-test-secret"
	streamWireUserId  = "22222222-2222-2222-2222-222222222222"
	streamWireGroupId = "the-readers-group"
)

// listeningStore is a store that can push. It captures the publish callback the
// server hands to ListenActivity, which is the seam Postgres's LISTEN loop
// occupies in production — so a test can inject an event exactly where a
// committed row would arrive, and can also see whether the loop was started at
// all.
type listeningStore struct {
	store.Store
	user    models.User
	listens chan func(models.ActivityEvent)
}

func newListeningStore(user models.User) *listeningStore {
	return &listeningStore{user: user, listens: make(chan func(models.ActivityEvent), 1)}
}

func (s *listeningStore) GetUserById(context.Context, string) (models.User, error) {
	return s.user, nil
}

func (s *listeningStore) ListenActivity(ctx context.Context, publish func(models.ActivityEvent)) error {
	s.listens <- publish
	<-ctx.Done()
	return nil
}

// startedListener returns the captured publish callback, failing if the loop
// was never started.
func (s *listeningStore) startedListener(t *testing.T) func(models.ActivityEvent) {
	t.Helper()

	select {
	case publish := <-s.listens:
		return publish
	case <-time.After(streamWireTimeout):
		t.Fatal("the activity listener was never started")
		return nil
	}
}

// newWiredServer builds the real server — every middleware, every route — with
// the activity feed switched on or off, and a store that can push.
func newWiredServer(t *testing.T, enabled bool) (*httptest.Server, *listeningStore) {
	t.Helper()

	t.Setenv("ACTIVITY_FEED_ENABLED", boolEnv(enabled))

	user := models.User{Id: streamWireUserId, Username: "reader", IsActive: true, Groups: []string{streamWireGroupId}}
	st := newListeningStore(user)

	// t.Context() is cancelled when the test ends, which stops the listener
	// goroutine with it.
	handler := server.NewServerWithProvider(t.Context(), st, titleprovider.Provider(nil), streamWireSecret)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv, st
}

func boolEnv(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func streamWireToken(t *testing.T) string {
	t.Helper()

	token, err := auth.MakeJWT(streamWireUserId, streamWireSecret, time.Hour)
	require.NoError(t, err, "failed to mint a test token")
	return token
}

// do runs one request with a deadline, so nothing in this file can block the
// suite. The response body is the caller's to close.
func do(t *testing.T, method, url, token string) *http.Response {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), streamWireTimeout)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	require.NoError(t, err, "failed to build the request")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "the request failed")
	return resp
}

func TestActivityStreamWiring(t *testing.T) {
	t.Run("the stream route is public to AuthMiddleware, the ticket route is not", func(t *testing.T) {
		srv, _ := newWiredServer(t, true)

		// No Authorization header on either request. The stream must reach its
		// own handler and be turned away by the ticket check; the ticket route
		// must never reach a handler at all.
		stream := do(t, http.MethodGet, srv.URL+"/activity/stream?ticket=nope", "")
		defer stream.Body.Close()
		streamBody := readBody(t, stream)

		require.Equal(t, http.StatusUnauthorized, stream.StatusCode,
			"an unredeemable ticket is a 401")
		require.Contains(t, streamBody, "ticket",
			"the stream must be rejected by its own ticket check, not by AuthMiddleware — "+
				"if AuthMiddleware answers this, EventSource can never connect at all")

		ticket := do(t, http.MethodPost, srv.URL+"/activity/stream-ticket", "")
		defer ticket.Body.Close()
		ticketBody := readBody(t, ticket)

		require.Equal(t, http.StatusUnauthorized, ticket.StatusCode,
			"minting a ticket without a token must be refused")
		require.NotContains(t, ticketBody, "ticket is invalid",
			"the ticket route must be stopped by AuthMiddleware, not reach the handler: "+
				"a public mint would let anyone hand themselves a stream")
	})

	t.Run("an event published where Postgres would publish it reaches a live client", func(t *testing.T) {
		srv, st := newWiredServer(t, true)
		publish := st.startedListener(t)

		token := streamWireToken(t)
		mint := do(t, http.MethodPost, srv.URL+"/activity/stream-ticket", token)
		defer mint.Body.Close()
		require.Equal(t, http.StatusOK, mint.StatusCode, "an authenticated caller must get a ticket")

		var minted struct {
			Ticket    string `json:"ticket"`
			ExpiresIn int    `json:"expiresIn"`
		}
		require.NoError(t, json.NewDecoder(mint.Body).Decode(&minted), "the ticket response must be JSON")
		require.NotEmpty(t, minted.Ticket, "the response must carry a ticket")
		require.Equal(t, 60, minted.ExpiresIn, "the response must say how long the ticket lives")

		stream := do(t, http.MethodGet, srv.URL+"/activity/stream?ticket="+minted.Ticket, "")
		defer stream.Body.Close()

		// Headers in hand means the handler has already subscribed: it writes
		// them after Subscribe and flushes them immediately. This is also the
		// assertion that the chain stayed flushable — RequestIdMiddleware wraps
		// every ResponseWriter, and without its Unwrap the handler answers 500
		// here instead of streaming.
		require.Equal(t, http.StatusOK, stream.StatusCode, "a valid ticket must open the stream")
		require.Equal(t, "text/event-stream", stream.Header.Get("Content-Type"),
			"the response must be a stream, not a buffered document")
		require.Equal(t, "no", stream.Header.Get("X-Accel-Buffering"),
			"the anti-buffering header must survive the middleware chain")

		titleName := "Arrival"
		publish(models.ActivityEvent{
			Id:        "aaaaaaaa-0000-4000-8000-00000000000a",
			Seq:       7,
			GroupId:   streamWireGroupId,
			GroupName: "Movie night",
			ActorId:   "another-member",
			ActorName: "Ana",
			Kind:      "title_added",
			TitleName: &titleName,
			CreatedAt: time.Now().UTC(),
		})

		frame := readFrameThroughChain(t, bufio.NewReader(stream.Body))

		require.Contains(t, frame, "id: 7\n", "the frame carries the event's seq")
		require.Contains(t, frame, "event: activity\n", "the frame is named activity")
		require.Contains(t, frame, `"kind":"title_added"`, "the frame carries the event itself, not just a signal")
	})

	t.Run("with the feed off there is no stream route and no listener", func(t *testing.T) {
		srv, st := newWiredServer(t, false)

		token := streamWireToken(t)

		stream := do(t, http.MethodGet, srv.URL+"/activity/stream?ticket=anything", "")
		defer stream.Body.Close()
		require.Equal(t, http.StatusNotFound, stream.StatusCode,
			"the stream route must be absent with the flag off, not present-and-empty")

		ticket := do(t, http.MethodPost, srv.URL+"/activity/stream-ticket", token)
		defer ticket.Body.Close()
		require.Equal(t, http.StatusNotFound, ticket.StatusCode,
			"the ticket route must be absent with the flag off")

		// The listener is started synchronously inside the flag branch, so if
		// the branch ran at all the callback is already in the channel. The
		// wait is only to catch a slow goroutine, not to be flaky about it.
		select {
		case <-st.listens:
			t.Fatal("with the flag off no listener may run: it holds a dedicated database connection for nothing")
		case <-time.After(100 * time.Millisecond):
		}
	})
}

// readBody drains a non-streaming response.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read the response body")
	return string(body)
}

// readFrameThroughChain reads one SSE message, skipping keep-alive comments.
// Every read is bounded by the request's own deadline.
func readFrameThroughChain(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	for {
		var message strings.Builder
		for {
			line, err := r.ReadString('\n')
			require.NoError(t, err, "the stream ended or timed out before a complete message arrived")
			if line == "\n" {
				break
			}
			message.WriteString(line)
		}
		if !strings.HasPrefix(message.String(), ":") {
			return message.String()
		}
	}
}
