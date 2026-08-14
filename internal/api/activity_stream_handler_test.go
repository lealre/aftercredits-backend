package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	activitycore "github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/activity"
	"github.com/lealre/movies-backend/internal/store"
)

// streamTestTimeout bounds every blocking read in this file. A stream that is
// never going to deliver must fail the test, not hang the suite, so nothing
// here reads without a deadline.
const streamTestTimeout = 5 * time.Second

// testPingInterval replaces the production 25s for the whole file: one ping
// has to be observable inside a test's patience.
const testPingInterval = 60 * time.Millisecond

const (
	streamUserId  = "11111111-1111-1111-1111-111111111111"
	streamGroupId = "group-the-reader-belongs-to"
)

// stubStreamStore satisfies store.Store by embedding the interface, so only the
// two methods these handlers reach need bodies. Any other call panics on the
// nil embedded interface, which is the point: it proves nothing else is
// touched.
type stubStreamStore struct {
	store.Store
	user    models.User
	userErr error
	feed    []models.ActivityEvent
}

func (s stubStreamStore) GetUserById(context.Context, string) (models.User, error) {
	return s.user, s.userErr
}

func (s stubStreamStore) GetActivityFeed(context.Context, string, *int64, int) ([]models.ActivityEvent, error) {
	return s.feed, nil
}

// refusingTickets is a TicketAuthority that refuses every redemption, standing
// in for the cases activity.TicketStore collapses into a single false: expired,
// unknown, already used. The store's own clock-driven expiry is pinned in
// internal/activity/ticket_test.go, where the clock is injectable; what matters
// here is what the handler does with a refusal, which must be the same answer
// whatever the reason.
type refusingTickets struct{}

func (refusingTickets) Issue(string) string          { return "irrelevant" }
func (refusingTickets) Redeem(string) (string, bool) { return "", false }
func (refusingTickets) TTL() time.Duration           { return time.Minute }

// unflushableWriter implements exactly the ResponseWriter contract and nothing
// else — no Flush, no Unwrap — which is what an HTTP/2 push writer or a badly
// written middleware looks like from the handler's side.
type unflushableWriter struct {
	header http.Header
	status int
	body   strings.Builder
}

func (w *unflushableWriter) Header() http.Header    { return w.header }
func (w *unflushableWriter) WriteHeader(status int) { w.status = status }
func (w *unflushableWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

type streamHarness struct {
	server  *httptest.Server
	hub     *activitycore.Hub
	tickets *activitycore.TicketStore
	api     *API
}

// newStreamHarness builds the two stream handlers (plus the feed handler, for
// the serialization comparison) over a real hub and a real ticket store.
//
// The stand-in for AuthMiddleware injects the user for every route EXCEPT
// GET /activity/stream, mirroring api.PublicPaths: the stream must authenticate
// itself from its ticket and must not quietly depend on a context user it will
// never have in production.
func newStreamHarness(t *testing.T, st store.Store) *streamHarness {
	t.Helper()

	hub := activitycore.NewHub()
	tickets := activitycore.NewTicketStore()

	a := NewAPI(st, nil)
	a.Stream = activity.NewStreamer(hub, tickets)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /activity/stream-ticket", a.IssueActivityStreamTicket)
	mux.HandleFunc("GET /activity/stream", a.StreamActivity)
	mux.HandleFunc("GET /activity", a.GetActivityFeed)

	user := models.User{Id: streamUserId, Username: "reader", IsActive: true, Groups: []string{streamGroupId}}
	withUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activity/stream" {
			r = r.WithContext(auth.WithUser(r.Context(), user))
		}
		mux.ServeHTTP(w, r)
	})

	server := httptest.NewServer(withUser)
	// Close blocks until every in-flight handler has returned, so by the time
	// this cleanup finishes no stream goroutine is still running.
	t.Cleanup(server.Close)

	return &streamHarness{server: server, hub: hub, tickets: tickets, api: a}
}

// mintTicket calls the real endpoint rather than the ticket store, so the tests
// that use a ticket also exercise the route that issues one.
func (h *streamHarness) mintTicket(t *testing.T) activity.StreamTicket {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), streamTestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.server.URL+"/activity/stream-ticket", nil)
	require.NoError(t, err, "failed to build the ticket request")

	resp, err := h.server.Client().Do(req)
	require.NoError(t, err, "the ticket request failed")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "minting a ticket must succeed for an authenticated user")

	var ticket activity.StreamTicket
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ticket), "the ticket response must be JSON")
	return ticket
}

// openStream connects to the stream and returns the response plus a reader over
// its body. The request carries a deadline and is cancelled by cleanup, so a
// stream that stops delivering ends the test instead of hanging it.
func (h *streamHarness) openStream(t *testing.T, ticket string) (*http.Response, *bufio.Reader) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), streamTestTimeout)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.server.URL+"/activity/stream?ticket="+ticket, nil)
	require.NoError(t, err, "failed to build the stream request")

	// The client returns as soon as the response headers arrive, which the
	// handler writes and flushes only after the ticket has been redeemed.
	resp, err := h.server.Client().Do(req)
	require.NoError(t, err, "the stream request failed")
	t.Cleanup(func() { resp.Body.Close() })

	return resp, bufio.NewReader(resp.Body)
}

// readMessage reads one SSE message — everything up to a blank line — as raw
// text, comments included.
func readMessage(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	var message strings.Builder
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "the stream ended or timed out before a complete message arrived")
		if line == "\n" {
			return message.String()
		}
		message.WriteString(line)
	}
}

// readActivityFrame reads the next activity message, skipping the ping comments
// that a live stream interleaves with real events.
func readActivityFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	for {
		message := readMessage(t, r)
		if !strings.HasPrefix(message, ":") {
			return message
		}
	}
}

// dataLine returns the JSON carried by an activity frame.
func dataLine(t *testing.T, frame string) string {
	t.Helper()

	for line := range strings.SplitSeq(strings.TrimRight(frame, "\n"), "\n") {
		if after, found := strings.CutPrefix(line, "data: "); found {
			return after
		}
	}
	t.Fatalf("no data line in frame %q", frame)
	return ""
}

// readAll drains and closes a non-streaming response body.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read the response body")
	return string(body)
}

// requireSubscribers waits for the hub to settle on n subscribers. Unsubscribe
// happens on the server's own schedule once the client goes away, so this polls
// rather than asserting instantly — but it is bounded, so a leak fails.
func requireSubscribers(t *testing.T, hub *activitycore.Hub, n int, msg string) {
	t.Helper()

	require.Eventually(t, func() bool { return hub.SubscriberCount() == n },
		streamTestTimeout, 5*time.Millisecond, msg)
}

func sampleEvent() models.ActivityEvent {
	titleId := "tt0111161"
	titleName := "The Shawshank Redemption"
	return models.ActivityEvent{
		Id:        "d7c0de00-0000-4000-8000-000000000001",
		Seq:       42,
		GroupId:   streamGroupId,
		GroupName: "Movie night",
		ActorId:   "someone-else",
		ActorName: "Ana",
		Kind:      "rating_added",
		TitleId:   &titleId,
		TitleName: &titleName,
		Payload:   map[string]any{"rating": float64(9)},
		CreatedAt: time.Date(2026, 8, 13, 10, 30, 0, 0, time.UTC),
	}
}

func TestActivityStreamHandlers(t *testing.T) {
	// Restored after every subtest has finished; each harness's httptest server
	// is closed by its own cleanup first, so no handler is still reading this.
	production := streamPingInterval
	streamPingInterval = testPingInterval
	t.Cleanup(func() { streamPingInterval = production })

	activeUser := models.User{Id: streamUserId, Username: "reader", IsActive: true, Groups: []string{streamGroupId}}

	t.Run("a ticket is minted for the authenticated caller and says when it dies", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})

		ticket := h.mintTicket(t)

		require.Len(t, ticket.Ticket, 64, "a ticket is 32 crypto/rand bytes, hex-encoded")
		require.Equal(t, 60, ticket.ExpiresIn, "expiresIn must report the store's real TTL in seconds")

		userId, ok := h.tickets.Redeem(ticket.Ticket)
		require.True(t, ok, "the minted ticket must be redeemable")
		require.Equal(t, streamUserId, userId, "the ticket must belong to the authenticated caller, nobody else")
	})

	t.Run("an unknown ticket is rejected with 401 and no stream", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})

		resp, _ := h.openStream(t, "not-a-ticket-anyone-issued")

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "an unknown ticket must not open a stream")
		require.NotContains(t, resp.Header.Get("Content-Type"), "text/event-stream",
			"a rejected request must not be answered as a stream")
		require.Zero(t, h.hub.SubscriberCount(), "a rejected request must not leave a subscriber behind")
	})

	t.Run("a redeemed ticket cannot be reused", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		first, _ := h.openStream(t, ticket.Ticket)
		require.Equal(t, http.StatusOK, first.StatusCode, "the first use of a ticket must succeed")
		first.Body.Close()
		requireSubscribers(t, h.hub, 0, "the first stream must be gone before the ticket is retried")

		second, _ := h.openStream(t, ticket.Ticket)
		require.Equal(t, http.StatusUnauthorized, second.StatusCode,
			"a ticket is single-use: replaying it must not open a second stream")
	})

	t.Run("a refused ticket is a 401 that does not say why it was refused", func(t *testing.T) {
		// Expired, unknown and already-used are one answer by design, so an
		// attacker cannot use the response to learn that a ticket string was
		// ever real. Compared body-to-body rather than asserted separately,
		// because "they are the same" is the actual property.
		unknown := newStreamHarness(t, stubStreamStore{user: activeUser})
		unknownResp, _ := unknown.openStream(t, "never-issued")
		unknownBody := readAll(t, unknownResp)

		expired := newStreamHarness(t, stubStreamStore{user: activeUser})
		expired.api.Stream = activity.NewStreamer(expired.hub, refusingTickets{})
		expiredResp, _ := expired.openStream(t, "issued-but-long-expired")
		expiredBody := readAll(t, expiredResp)

		require.Equal(t, http.StatusUnauthorized, expiredResp.StatusCode, "an expired ticket must be a 401")
		require.Equal(t, unknownResp.StatusCode, expiredResp.StatusCode,
			"expired and unknown tickets must get the same status")
		require.Equal(t, unknownBody, expiredBody,
			"expired and unknown tickets must get the same body — the difference must not leak")
	})

	t.Run("a store failure while resolving groups is a 500, not a 401", func(t *testing.T) {
		// CONVENTIONS §3: the error is checked before the boolean. Folding them
		// together would answer a database outage with "bad ticket" and send
		// every client into a reconnect loop that cannot succeed.
		h := newStreamHarness(t, stubStreamStore{userErr: errors.New("connection refused")})
		ticket := h.mintTicket(t)

		resp, _ := h.openStream(t, ticket.Ticket)

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode,
			"a store failure must be a 500, not a 401 the client will retry forever")
	})

	t.Run("a deactivated user cannot redeem a ticket into a stream", func(t *testing.T) {
		inactive := activeUser
		inactive.IsActive = false

		h := newStreamHarness(t, stubStreamStore{user: inactive})
		ticket := h.mintTicket(t)

		resp, _ := h.openStream(t, ticket.Ticket)

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"a ticket outliving its user's account must not still open a stream")
		require.Zero(t, h.hub.SubscriberCount(), "a rejected user must not be subscribed")
	})

	t.Run("the stream headers arrive before any event does", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		// Nothing has been published, so receiving a response at all proves the
		// handler flushed its headers instead of waiting for something to send.
		resp, _ := h.openStream(t, ticket.Ticket)

		require.Equal(t, http.StatusOK, resp.StatusCode, "a valid ticket must open the stream")
		require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"),
			"EventSource only accepts text/event-stream")
		require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"),
			"a cached stream is a stream that never updates")
		require.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"),
			"nginx buffers by default, which makes the stream connect and deliver nothing")
		require.Equal(t, "keep-alive", resp.Header.Get("Connection"),
			"the stream is held open, not closed after the first write")
	})

	t.Run("a published event arrives as an activity frame", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		_, body := h.openStream(t, ticket.Ticket)
		requireSubscribers(t, h.hub, 1, "the connected client must be subscribed before anything is published")

		event := sampleEvent()
		h.hub.Publish(event)

		frame := readActivityFrame(t, body)

		require.Contains(t, frame, "id: 42\n", "the frame must carry the event's seq as its id")
		require.Contains(t, frame, "event: activity\n", "the frame must be named so clients can filter on it")

		var pushed activity.Event
		require.NoError(t, json.Unmarshal([]byte(dataLine(t, frame)), &pushed),
			"the data line must be the event DTO as JSON")
		require.Equal(t, event.Id, pushed.Id, "the pushed event must be the one published")
		require.Equal(t, "rating_added", pushed.Kind, "the pushed event must carry its kind")
		require.Equal(t, "Movie night", pushed.GroupName, "the pushed event must carry its group name")
	})

	t.Run("an idle stream sends a ping so proxies do not cut it", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		_, body := h.openStream(t, ticket.Ticket)

		// Nothing is ever published on this stream: the only thing that can
		// arrive is the keep-alive.
		require.Equal(t, ":ping\n", readMessage(t, body),
			"an idle stream must write a comment line before a proxy's read timeout")
	})

	t.Run("a disconnected client is unsubscribed", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		resp, _ := h.openStream(t, ticket.Ticket)
		requireSubscribers(t, h.hub, 1, "the connected client must be subscribed")

		resp.Body.Close()

		requireSubscribers(t, h.hub, 0,
			"a dropped connection must unsubscribe, or the hub leaks a subscriber and a channel per lost client")
	})

	t.Run("a ResponseWriter that cannot flush is a 500 that does not burn the ticket", func(t *testing.T) {
		h := newStreamHarness(t, stubStreamStore{user: activeUser})
		ticket := h.mintTicket(t)

		w := &unflushableWriter{header: http.Header{}}
		req := httptest.NewRequest(http.MethodGet, "/activity/stream?ticket="+ticket.Ticket, nil)

		h.api.StreamActivity(w, req)

		require.Equal(t, http.StatusInternalServerError, w.status,
			"serving a stream nobody can receive is worse than refusing it")
		require.Zero(t, h.hub.SubscriberCount(), "a refused request must not subscribe")

		_, ok := h.tickets.Redeem(ticket.Ticket)
		require.True(t, ok,
			"the flushability check must run before the ticket is redeemed, or a retry fails too")
	})

	t.Run("the pushed frame and the REST feed are the same bytes for the same event", func(t *testing.T) {
		// Spec test 11. Pushing data instead of a signal costs one fact with
		// two serializations; this is what stops them drifting.
		event := sampleEvent()
		h := newStreamHarness(t, stubStreamStore{user: activeUser, feed: []models.ActivityEvent{event}})
		ticket := h.mintTicket(t)

		_, body := h.openStream(t, ticket.Ticket)
		requireSubscribers(t, h.hub, 1, "the client must be subscribed before the event is published")
		h.hub.Publish(event)
		pushed := dataLine(t, readActivityFrame(t, body))

		ctx, cancel := context.WithTimeout(t.Context(), streamTestTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.server.URL+"/activity", nil)
		require.NoError(t, err, "failed to build the feed request")
		feedResp, err := h.server.Client().Do(req)
		require.NoError(t, err, "the feed request failed")
		defer feedResp.Body.Close()

		var feed struct {
			Events []json.RawMessage `json:"events"`
		}
		require.NoError(t, json.NewDecoder(feedResp.Body).Decode(&feed), "the feed response must be JSON")
		require.Len(t, feed.Events, 1, "the stubbed feed must carry exactly the published event")

		require.Equal(t, string(feed.Events[0]), pushed,
			"one fact, one serialization: the stream frame and the feed element must be byte-identical")
	})
}
