package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/metrics"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/server"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// These tests are about the CHAIN, not about the middleware. internal/metrics
// pins what the middlewares do; what nothing pinned until now is where they sit
// relative to each other in the handler NewServerWithProvider actually returns.
//
// That distinction is not academic. http.ServeMux assigns Request.Pattern to
// the request object it is handed, and every middleware that calls
// r.WithContext hands it a NEW object. So RouteCaptureMiddleware reads a
// non-empty pattern only if it is the innermost wrapper, directly around the
// mux — moved one wrapper outward it reads "" for 100% of traffic and every
// request is silently labelled route="unmatched". A test that composes its own
// chain (as internal/metrics' does, deliberately, for the middleware itself)
// cannot see that: it builds the arrangement it means to test. This one drives
// the real constructor instead, so the arrangement IS the thing under test.
//
// The store is the wiring stub from activity_stream_test.go, which satisfies
// the one store method both AuthMiddleware and GET /users/{id} call.
func newMetricsWiredServer(t *testing.T) (*httptest.Server, *metrics.Metrics) {
	t.Helper()

	t.Setenv("ACTIVITY_FEED_ENABLED", "true")

	user := models.User{Id: streamWireUserId, Username: "reader", IsActive: true}
	st := newListeningStore(user)

	// The test's own registry, so the exposition read below is written by the
	// chain under test and nobody else's.
	m := metrics.New(nil)
	handler := server.NewServerWithProvider(t.Context(), st, titleprovider.Provider(nil), streamWireSecret, m)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv, m
}

// exposition renders the registry the way the metrics listener would serve it.
// It goes straight to the handler rather than through the test server, so the
// scrape itself is never a request the middleware could count.
func exposition(t *testing.T, m *metrics.Metrics) string {
	t.Helper()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// eventuallyExposes polls until the exposition contains want.
//
// Polling rather than asserting once, because the middleware records AFTER the
// handler returns: the client can observe its own response before the server
// has finished counting it. A single immediate assertion would be a flaky
// test, not a strict one. Same helper as internal/metrics/middleware_test.go.
func eventuallyExposes(t *testing.T, m *metrics.Metrics, want, msg string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return strings.Contains(exposition(t, m), want)
	}, 2*time.Second, 10*time.Millisecond, msg)
}

// The route label must be the PATTERN of the route that served the request.
//
// A non-empty pattern here means the capture middleware ran directly around the
// mux, on the same request object the mux wrote the pattern onto. Wrapped one
// position further out, this test fails: the mux would assign the pattern to
// the copy RequestTimeoutMiddleware made, and the label would read "unmatched".
func TestMetricsWiring_LabelsTheMatchedRoutePattern(t *testing.T) {
	srv, m := newMetricsWiredServer(t)

	resp := do(t, http.MethodGet, srv.URL+"/users/"+streamWireUserId, streamWireToken(t))
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"the request must reach its handler; anything else and this test is not exercising the chain")

	eventuallyExposes(t, m, `route="GET /users/{id}"`,
		"the label must be the matched route pattern — "+
			"an empty pattern means RouteCaptureMiddleware is not directly around the mux")

	// The converse, asserted once the record above has been observed: this
	// server has served exactly one request, and it was matched, so nothing may
	// have been reported as unmatched. Nothing to poll for — the middleware
	// records one label per request, not both.
	require.NotContains(t, exposition(t, m), `route="unmatched"`,
		"a request that matched a route must not be labelled unmatched")
}
