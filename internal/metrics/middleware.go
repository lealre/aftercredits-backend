package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// unmatchedRoute labels a request whose route could not be determined: one
// that never reached the mux (rejected by a middleware above it, so the
// handler never ran) or that matched no pattern (404).
//
// It is a fixed string rather than the request path on purpose. The path is
// caller-controlled, so a label carrying it would let anyone grow the time
// series without limit — and a 401 on an unknown path is better described as
// "unmatched" than by echoing back whatever was asked for.
const unmatchedRoute = "unmatched"

// methodLabel returns a bounded label for the request method.
//
// r.Method is whatever token the caller sent, and HTTP permits arbitrary
// tokens, so using it raw would let anyone mint unlimited time series — the
// same hazard that keeps the raw request path out of the route label, and a
// sharper one on a Raspberry Pi, where an unbounded label set is memory rather
// than noise. Anything outside the standard set is reported as "other", which
// still answers "were these ordinary requests or something else" without
// letting the caller choose the label.
func methodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "other"
	}
}

type contextKey string

const metricsMetaKey contextKey = "metricsMeta"

// metricsMeta is a mutable per-request holder, for the same reason requestMeta
// exists in internal/server: r.WithContext returns a COPY of the request, so a
// route written onto the request the mux sees is invisible to the outer
// middleware that records it. A pointer's target propagates back up the chain;
// the request does not.
type metricsMeta struct {
	route string
}

// RouteCaptureMiddleware records the matched route pattern for the metrics
// middleware to read once the handler returns.
//
// It must be the INNERMOST wrapper — directly around the mux. http.ServeMux
// sets Request.Pattern on the request object it is given, and every middleware
// that calls r.WithContext replaces that object, so wrapped further out it
// would always read an empty pattern and every request would look unmatched.
//
// It is a no-op when no metrics meta is in the context, so it can be installed
// unconditionally.
func RouteCaptureMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		// Read AFTER the handler: the mux assigns Pattern while serving.
		if meta, ok := r.Context().Value(metricsMetaKey).(*metricsMeta); ok {
			meta.route = r.Pattern
		}
	})
}

// Middleware records request counts, latency and in-flight depth.
//
// It is placed just inside the request-id middleware so it observes every
// request, including ones auth rejects before the mux is reached. Those have
// no route to report and are labelled unmatchedRoute.
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			meta := &metricsMeta{}
			r = r.WithContext(context.WithValue(r.Context(), metricsMetaKey, meta))

			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			m.inFlight.Inc()
			// Deferred, not sequential: a panicking handler unwinds past
			// everything below this line, and the gauge must not be left
			// stuck above zero by a request that will never finish.
			defer m.inFlight.Dec()

			start := time.Now()
			next.ServeHTTP(recorder, r)

			route := meta.route
			if route == "" {
				route = unmatchedRoute
			}
			method := methodLabel(r.Method)
			m.requests.WithLabelValues(method, route, strconv.Itoa(recorder.status)).Inc()
			m.duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
		})
	}
}

// statusRecorder wraps a ResponseWriter to remember the status code written.
//
// Unwrap is REQUIRED, not decorative. Embedding http.ResponseWriter promotes
// only that interface's methods, so without it this type stops satisfying
// http.Flusher and the activity SSE stream silently never reaches the client.
// The same hazard, and the same fix, are documented on responseRecorder in
// internal/server/middleware.go.
//
// It is a small deliberate duplicate of that type: internal/metrics must not
// import internal/server, which imports this package.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
