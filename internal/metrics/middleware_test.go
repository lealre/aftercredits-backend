package metrics_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/metrics"
)

// buildServer wires the two middlewares exactly as internal/server does, so
// the test exercises the real chain shape rather than an approximation.
func buildServer(t *testing.T, routes func(*http.ServeMux)) (*httptest.Server, *metrics.Metrics) {
	t.Helper()
	m := metrics.New(nil)
	mux := http.NewServeMux()
	routes(mux)

	var h http.Handler = mux
	h = metrics.RouteCaptureMiddleware(h) // innermost, directly around the mux
	h = m.Middleware()(h)

	return httptest.NewServer(h), m
}

func expose(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

// eventuallyExposes polls until the exposition contains want.
//
// Polling rather than asserting once, because the middleware records AFTER the
// handler returns: the client can observe its own response before the server
// has finished counting it. A single immediate assertion would be a flaky
// test, not a strict one.
func eventuallyExposes(t *testing.T, m *metrics.Metrics, want, msg string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return strings.Contains(expose(t, m), want)
	}, 2*time.Second, 10*time.Millisecond, msg)
}

// neverExposes asserts the exposition does not contain unwanted, after giving
// the recorder the same grace period the polling helper uses — so this cannot
// pass merely by looking too early.
func neverExposes(t *testing.T, m *metrics.Metrics, unwanted, msg string) {
	t.Helper()
	require.Never(t, func() bool {
		return strings.Contains(expose(t, m), unwanted)
	}, 200*time.Millisecond, 20*time.Millisecond, msg)
}

// The route label must be the registered PATTERN, not the concrete path.
// Labelling by path would make every id its own time series.
func TestMiddleware_LabelsByRoutePattern(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/users/abc-123")
	require.NoError(t, err)
	resp.Body.Close()

	eventuallyExposes(t, m, `route="GET /users/{id}"`, "route label must be the pattern")
	// The other half of the allowlist: a standard method passes through as
	// itself. Without this the bucketing test alone is satisfied by a
	// methodLabel that returns "other" for EVERY token — the route label would
	// still carry GET from the registered pattern, so nothing else in the suite
	// would notice, and the "is this ordinary traffic" question would be
	// unanswerable for all traffic.
	eventuallyExposes(t, m, `method="GET"`, "a standard method must pass through as itself")
	neverExposes(t, m, "abc-123", "the concrete id must never reach a label")
}

// A request that matches no pattern still has to be recorded, under a fixed
// label — otherwise a caller could grow the series count by requesting
// nonsense paths.
func TestMiddleware_UnmatchedRouteIsLabelled(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /known", func(w http.ResponseWriter, r *http.Request) {})
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/definitely-not-a-route")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	eventuallyExposes(t, m, `route="unmatched"`, "unmatched requests get the fixed label")
	eventuallyExposes(t, m, `status="404"`, "the status is still recorded")
	neverExposes(t, m, "definitely-not-a-route", "the raw path must never reach a label")
}

// The other half of the cardinality rule. `route` is bounded because the mux
// decides it; `method` is bounded only if the middleware refuses to take the
// caller's word for it. HTTP permits arbitrary method tokens, so labelling by
// r.Method raw would let a caller mint a permanent time series per token — and
// this process runs on a Raspberry Pi, where that is memory, not just noise.
//
// The token is bucketed to "other" rather than dropped, so the question "is
// this ordinary traffic or something else" stays answerable.
//
// http.Get cannot send a custom method, hence the explicit request.
func TestMiddleware_NonStandardMethodIsBucketed(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		// Registered without a method so it matches any method token.
		mux.HandleFunc("/anything", func(w http.ResponseWriter, r *http.Request) {})
	})
	defer srv.Close()

	req, err := http.NewRequest("FOOBAR", srv.URL+"/anything", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	eventuallyExposes(t, m, `method="other"`, "a non-standard method token must be bucketed")
	neverExposes(t, m, "FOOBAR", "the raw method token must never reach a label")
}

// A middleware between the metrics middleware and the mux can reject a request
// before the handler ever runs — AuthMiddleware does exactly this, and a 401
// rate is precisely what we want to see. Those requests must still be counted,
// even though their route is unknowable.
//
// So the rejecter is composed INSIDE the metrics middleware, which is where
// AuthMiddleware sits in the real chain (MetricsMiddleware's comment: it "is
// placed just inside the request-id middleware so it observes every request,
// including ones auth rejects before the mux is reached"). The converse is
// worth stating because it is not a defect in the middleware: a rejecter ABOVE
// metrics that never calls next is invisible to it, and no implementation
// could record it — the recorder's own code never runs. Composing reject
// outside would test that impossibility, not the middleware.
func TestMiddleware_CountsRequestsRejectedBeforeTheMux(t *testing.T) {
	m := metrics.New(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /private", func(w http.ResponseWriter, r *http.Request) {})

	reject := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}

	var h http.Handler = mux
	h = metrics.RouteCaptureMiddleware(h) // directly around the mux
	h = reject(h)                         // rejects before the mux is reached
	h = m.Middleware()(h)                 // outside it, so it observes the 401

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/private")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	eventuallyExposes(t, m, `status="401"`, "requests rejected above the handler must still be counted")
	eventuallyExposes(t, m, `route="unmatched"`, "their route is not knowable, so the label says so")
}

// The gauge must return to zero when a request finishes.
func TestMiddleware_InFlightReturnsToZero(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {})
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	require.NoError(t, err)
	resp.Body.Close()

	eventuallyExposes(t, m, "aftercredits_http_requests_in_flight 0",
		"in-flight must return to zero once the request completes")
}

// flusherFor asks the writer the same question api.flusherFor asks in
// internal/api/activity_stream_handler.go:150: does anything in the Unwrap
// chain underneath flush? That loop is what the activity SSE handler runs on
// every request, so this test mirrors the production question rather than an
// approximation of it.
//
// A direct w.(http.Flusher) assertion CANNOT be used here: the wrapper promotes
// only the three http.ResponseWriter methods, so the assertion is false on a
// writer that can in fact flush — it reports "flusher=false" with AND without
// Unwrap, and a test written that way detects nothing. (The mutation check in
// the task report records it failing identically in both states.) Walking the
// chain is the protocol http.ResponseController uses, and the one api.flusherFor
// uses.
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

// THE highest-value test here. Embedding http.ResponseWriter promotes only
// that interface's methods, so a wrapper without Unwrap stops satisfying
// http.Flusher — and the activity SSE stream then silently never reaches the
// client. See spec §6.4 and internal/server/middleware.go:91.
func TestMiddleware_WrappedWriterStillFlushes(t *testing.T) {
	m := metrics.New(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		// The answer travels in the response body rather than a captured
		// variable: the handler runs on another goroutine, so a shared bool is
		// a data race under -race. The HTTP round trip carries it across safely.
		flusher, isFlusher := flusherFor(w)
		fmt.Fprintf(w, "flusher=%t", isFlusher)
		if isFlusher {
			flusher.Flush()
		}
	})

	var h http.Handler = mux
	h = metrics.RouteCaptureMiddleware(h)
	h = m.Middleware()(h)

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/stream")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, "flusher=true", string(body),
		"the handler must still see an http.Flusher through the metrics wrapper; "+
			"without Unwrap the activity SSE stream breaks silently")
}

// The SSE stream is a response held open for as long as a tab stays on the
// page. Timing it measures how long someone left the app open, not latency —
// every disconnect would land a multi-minute sample in the +Inf bucket and drag
// the p99 panel with it, and the in-flight gauge would count idle tabs rather
// than work in progress. This pins the exemption so a refactor cannot quietly
// restore it.
func TestStreamIsCountedButNotTimed(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /activity/stream", func(w http.ResponseWriter, r *http.Request) {})
		mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {})
	})
	defer srv.Close()

	res, err := http.Get(srv.URL + "/activity/stream")
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())

	body := expose(t, m)
	require.Contains(t, body, `aftercredits_http_requests_total{method="GET",route="GET /activity/stream"`,
		"the stream must still be counted: one increment per connection is right")
	require.NotContains(t, body, `aftercredits_http_request_duration_seconds_count{method="GET",route="GET /activity/stream"`,
		"the stream must NOT be timed — its duration is how long a tab stayed open")

	// The counterpart. Without it, this test passes just as happily if the
	// histogram stopped recording anything at all.
	res, err = http.Get(srv.URL + "/groups")
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())

	require.Contains(t, expose(t, m), `aftercredits_http_request_duration_seconds_count{method="GET",route="GET /groups"`,
		"an ordinary request must still be timed")
}

// A stream is not a slow request, it is a subscription. Counting it in the HTTP
// instruments answered the wrong question; these answer the right one.
func TestStreamGetsItsOwnInstruments(t *testing.T) {
	srv, m := buildServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /activity/stream", func(w http.ResponseWriter, r *http.Request) {})
	})
	defer srv.Close()

	res, err := http.Get(srv.URL + "/activity/stream")
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())

	body := expose(t, m)
	require.Contains(t, body, "aftercredits_activity_streams_total 1",
		"an opened stream must be counted")
	require.Contains(t, body, "aftercredits_activity_stream_duration_seconds_count 1",
		"a closed stream must have its lifetime recorded")
	require.Contains(t, body, "aftercredits_activity_streams_active 0",
		"the active gauge must return to zero once the stream closes")
	require.NotContains(t, body, "aftercredits_http_requests_in_flight 1",
		"a stream must never be left in the HTTP in-flight gauge")
}
