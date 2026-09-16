package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// unmatchedRoute labels a request that never reached the mux (rejected by a
// middleware above it) or matched no pattern (404).
//
// A fixed string rather than the request path: the path is caller-controlled,
// so a label carrying it would let anyone grow the series count without limit.
const unmatchedRoute = "unmatched"

// streamPath is the one route whose response is deliberately long-lived. It is
// matched by path rather than by the recorded route label because the label is
// only known after the handler runs, and the decision is needed before it.
const streamPath = "/activity/stream"

func isStream(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == streamPath
}

// methodLabel bounds the method label to the standard set, reporting anything
// else as "other".
//
// HTTP permits arbitrary method tokens, so using r.Method raw would let a
// caller mint unlimited time series — the same hazard that keeps the raw path
// out of the route label, and a sharper one on a Pi, where an unbounded label
// set is memory rather than noise.
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

			// The SSE stream is a response held open for as long as a browser
			// tab stays on the page, exempted from RequestTimeoutMiddleware for
			// that reason. Timing it measures how long someone left the app
			// open, which is not latency: every disconnect would land a
			// multi-minute observation in the +Inf bucket (DefBuckets stop at
			// 10s), and the in-flight gauge would count connected tabs rather
			// than work in progress. Both would read as a server in trouble.
			//
			// It is still counted in requests_total, where one increment per
			// connection is exactly right.
			timed := !isStream(r)

			if timed {
				m.inFlight.Inc()
				// Deferred, not sequential: a panicking handler unwinds past
				// everything below this line, and the gauge must not be left
				// stuck above zero by a request that will never finish.
				defer m.inFlight.Dec()
			}

			start := time.Now()
			next.ServeHTTP(recorder, r)

			route := meta.route
			if route == "" {
				route = unmatchedRoute
			}
			method := methodLabel(r.Method)
			m.requests.WithLabelValues(method, route, strconv.Itoa(recorder.status)).Inc()
			if timed {
				m.duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
			}
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
