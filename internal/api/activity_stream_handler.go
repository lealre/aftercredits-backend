package api

import (
	"io"
	"net/http"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/activity"
)

// streamPingInterval is how often an otherwise idle stream writes a comment
// line. Proxies cut a connection that has been silent for their read timeout
// (nginx defaults to 60s), and a cut stream looks to the client like a server
// that stopped caring, so the stream proves it is alive well inside that
// window.
//
// A var, not a const, so a test can shorten it: waiting 25 seconds to observe
// one ping is not a test, it is a hang.
var streamPingInterval = 25 * time.Second

// IssueActivityStreamTicket exchanges the caller's Bearer token for a
// single-use ticket the SSE endpoint accepts. This route is deliberately not
// in PublicPaths — it is the authentication step the stream route delegates
// to, so it must run behind AuthMiddleware like every other private route.
func (api *API) IssueActivityStreamTicket(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.GetUserFromContext(r.Context())

	respondWithJSON(w, http.StatusOK, api.Stream.IssueTicket(currentUser.Id))
}

// StreamActivity holds an SSE connection open and writes the caller's group
// activity to it as it happens.
//
// It authenticates by ticket rather than Bearer token, which is why
// "GET /activity/stream" is in PublicPaths: EventSource cannot set headers, so
// AuthMiddleware would reject every stream before this handler ran. Public to
// the middleware does not mean unauthenticated — an unredeemable ticket is a
// 401 here.
func (api *API) StreamActivity(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	// Checked first, and without writing anything: a ticket is single-use, so
	// redeeming one and then discovering the response cannot be streamed would
	// burn it and make the client's retry fail too. flusherFor is used instead
	// of http.ResponseController.Flush because that commits a 200 the moment
	// it is called, which would take the 401 below off the table.
	flusher, ok := flusherFor(w)
	if !ok {
		logger.Printf("ERROR: the activity stream needs a flushable ResponseWriter, got %T", w)
		respondWithError(w, http.StatusInternalServerError, "Streaming is not supported")
		return
	}

	subscriber, err := api.Stream.OpenStream(api.Db, r.Context(), r.URL.Query().Get("ticket"))
	if err != nil {
		if code, ok := activity.ErrorMap[err]; ok {
			respondWithError(w, code, err.Error())
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}
	// Exactly one unsubscribe, on every exit path. Without it the hub keeps a
	// subscriber and its channel for every client that ever disconnected, and
	// a dropped connection is the normal way a stream ends.
	defer api.Stream.CloseStream(subscriber)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// nginx buffers a proxied response by default, which makes the stream
	// connect and then deliver nothing — the failure the design doc calls out
	// as silent. Setting this from the application means a wrong proxy config
	// cannot cause it.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// EventSource fires `open` on the response headers, and the client takes
	// its snapshot then, so the headers must not wait for the first event.
	flusher.Flush()

	ping := time.NewTicker(streamPingInterval)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			// The client went away (closed tab, dropped network, shutdown).
			// Returning runs the deferred unsubscribe; staying would hold a
			// goroutine and a hub entry for a connection nobody is reading.
			return

		case event, ok := <-subscriber.Events:
			if !ok {
				// The hub closed us out. Nothing further will arrive.
				return
			}
			frame, err := activity.StreamFrame(event)
			if err != nil {
				// One unserializable event must not end a stream that is
				// otherwise healthy; the client's next snapshot still has it.
				logger.Printf("ERROR: dropping unserializable activity event %q: %v", event.Id, err)
				continue
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			// Without this the client sees nothing until the write buffer
			// fills, which for a low-traffic feed is effectively never.
			flusher.Flush()

		case <-ping.C:
			// A comment line: clients ignore it, proxies see traffic.
			if _, err := io.WriteString(w, ":ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// flusherFor finds something in w's Unwrap chain that can flush, without
// writing anything.
//
// The direct w.(http.Flusher) assertion is not enough: every request is
// wrapped by RequestIdMiddleware's responseRecorder (and mutating ones by the
// activity recorder), which implement only the three ResponseWriter methods,
// so the assertion fails on a writer that can in fact flush. This walks the
// same Unwrap chain http.ResponseController does, but as a question rather
// than as a write — ResponseController.Flush would commit a 200 and take the
// 401 the caller may still need to send off the table.
func flusherFor(w http.ResponseWriter) (http.Flusher, bool) {
	for {
		if flusher, ok := w.(http.Flusher); ok {
			return flusher, true
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, false
		}
		w = unwrapper.Unwrap()
	}
}
