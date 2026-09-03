package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/store"
)

type contextKey string

const requestIdKey contextKey = "requestId"
const requestMetaKey contextKey = "requestMeta"

// maxRequestBodyBytes caps every request body read through RequestIdMiddleware.
const maxRequestBodyBytes = 64 << 10

// requestMeta is a mutable per-request holder so a middleware that runs later
// (AuthMiddleware) can record the authenticated user id for the completion log
// line emitted by a middleware that runs earlier (RequestIdMiddleware) —
// context values do not propagate back up the chain, but a pointer's target
// does.
type requestMeta struct {
	userId string
}

// clientIP returns the caller's address for logging. nginx sets X-Real-IP to
// the real client after its real_ip config resolves the Cloudflare header;
// falling back to RemoteAddr keeps a sane value in local/dev runs. The value is
// sanitized before it reaches a log line.
func clientIP(r *http.Request) string {
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return sanitizeForLog(xr, 45)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return sanitizeForLog(host, 45)
}

// sanitizeForLog strips characters that could forge a log line — CR and LF
// above all, since a decoded %0a would otherwise inject a whole fake entry —
// and truncates to max bytes so an attacker cannot choose the log volume.
func sanitizeForLog(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r < 0x20 {
			return -1
		}
		return r
	}, s)
	if len(s) > max {
		return s[:max]
	}
	return s
}

////////////////////////////////////////////////////////////////////////////
//  LOGGER MIDDLEWARE
////////////////////////////////////////////////////////////////////////////

// Creates a unique 5-character identifier
func generateRequestId() string {
	bytes := make([]byte, 3) // 3 bytes = 6 hex chars, we'll take first 5
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:5]
}

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rr *responseRecorder) WriteHeader(statusCode int) {
	rr.statusCode = statusCode
	rr.ResponseWriter.WriteHeader(statusCode)
}

// Unwrap exposes the writer underneath, so capabilities this recorder does not
// implement — http.Flusher above all — stay reachable through it. Embedding an
// interface promotes only that interface's methods, so without this a wrapped
// ResponseWriter silently stops being flushable, and the SSE handler either
// refuses to serve or serves a stream that never reaches the client.
//
// This is the chain http.ResponseController walks, and the one
// api.flusherFor walks.
func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
}

/*
RequestIdMiddleware creates a unique request ID for each request and stores it in the context.
Creates a logger with the request ID prefixed to all log messages and stores it in the context.
- Log prefix format: [RequestId][Method:Endpoint]
- Logs when recive a request
- Logs when returns the response with time the request take and status code

Handlers can retrieve the logger using logx.FromContext(r.Context()).
Returns an http.Handler that wraps the next handler.
*/
func RequestIdMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cap the request body every handler will read. 64 KiB is comfortably
		// above any legitimate request this API takes (the largest is a title
		// import) and turns an unbounded-JSON memory-exhaustion attempt into a
		// 413. This runs first, so it protects every route including login and
		// registration. The SSE stream is a body-less GET, so the cap is inert
		// for it.
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		requestId := generateRequestId()
		startTime := time.Now()
		ip := clientIP(r)

		// The path is kept OUT of the log prefix and sanitized: it is
		// attacker-controlled, and the previous code spliced the DECODED path
		// into the prefix, so a request to /x%0a... injected a newline and a
		// forged log line before any auth ran. EscapedPath keeps it in its
		// wire form; sanitizeForLog strips any residual control bytes and caps
		// the length.
		method := sanitizeForLog(r.Method, 8)
		path := sanitizeForLog(r.URL.EscapedPath(), 256)

		logger := log.New(os.Stdout, "["+requestId+"] - ", log.LstdFlags)
		logger.Printf("Request received: %s %s from %s", method, path, ip)

		meta := &requestMeta{}
		ctx := context.WithValue(r.Context(), requestIdKey, requestId)
		ctx = context.WithValue(ctx, requestMetaKey, meta)
		ctx = logx.WithLogger(ctx, logger)
		r = r.WithContext(ctx)

		recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(startTime)
		userId := meta.userId
		if userId == "" {
			userId = "-"
		}
		// A single completion line per request, carrying who and from where, so
		// a sweep is attributable. A forbidden/unauthorized outcome is logged at
		// WARN so auth failures are greppable.
		level := "INFO"
		if recorder.statusCode == http.StatusUnauthorized || recorder.statusCode == http.StatusForbidden {
			level = "WARN"
		}
		if duration > time.Second {
			logger.Printf("%s: %s %s -> %d in %.2fs (ip=%s user=%s)", level, method, path, recorder.statusCode, duration.Seconds(), ip, userId)
		} else {
			logger.Printf("%s: %s %s -> %d in %dms (ip=%s user=%s)", level, method, path, recorder.statusCode, duration.Milliseconds(), ip, userId)
		}
	})
}

////////////////////////////////////////////////////////////////////////////
//  REQUEST TIMEOUT MIDDLEWARE
////////////////////////////////////////////////////////////////////////////

// requestTimeout bounds how long any ordinary request's context lives, so a
// slow query cannot pin a pool connection indefinitely.
const requestTimeout = 10 * time.Second

// RequestTimeoutMiddleware puts a deadline on the request context so database
// work started by a handler is cancelled if it overruns.
//
// The SSE activity stream is exempt BY PATH: it is a deliberately long-lived
// response, and a 10-second deadline on its context would tear it down. It has
// its own lifetime handling instead.
func RequestTimeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/activity/stream" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

////////////////////////////////////////////////////////////////////////////
//  AUTHENTICATION MIDDLEWARE
////////////////////////////////////////////////////////////////////////////

func AuthMiddleware(tokenSecret string, db store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Skip authentication for public endpoints
			if api.PublicPaths[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token
			tokenString, err := auth.GetBearerToken(r.Header)
			if err != nil {
				if _, ok := auth.ErrorsMap[err]; ok {
					api.RespondWithUnauthorized(w, err)
					return
				}
				http.Error(w, "Missing or invalid token", http.StatusUnauthorized)
				return
			}

			// Validate token
			userId, tokenVersion, err := auth.ValidateJWT(tokenString, tokenSecret)
			if err != nil {
				if _, ok := auth.ErrorsMap[err]; ok {
					api.RespondWithUnauthorized(w, err)
					return
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check the error before the boolean (CONVENTIONS §3). A store
			// failure leaves userDb as the zero models.User, whose IsActive is
			// false, so folding the two cases together reported every transient
			// database error as "invalid or inactive user" and logged nothing —
			// silently logging out every caller for as long as the database was
			// unreachable.
			userDb, err := db.GetUserById(r.Context(), userId)
			if err != nil && !errors.Is(err, store.ErrRecordNotFound) {
				logx.FromContext(r.Context()).Printf("ERROR: %v", err)
				http.Error(w, "Unexpected error occurred", http.StatusInternalServerError)
				return
			}
			// Genuinely unknown or deactivated user: unchanged 401 body/status.
			if errors.Is(err, store.ErrRecordNotFound) || !userDb.IsActive {
				http.Error(w, "Invalid or inactive user", http.StatusUnauthorized)
				return
			}

			// Revocation check: a token is only valid while it carries the
			// user's current token_version. A password change or an explicit
			// "log out everywhere" bumps the row, which invalidates every token
			// minted before it without any server-side session store. The row
			// is already loaded above, so this costs no extra query.
			if tokenVersion != userDb.TokenVersion {
				http.Error(w, "Token has been revoked", http.StatusUnauthorized)
				return
			}

			// Record the authenticated user id for the request-completion log
			// line (see requestMeta), so every request is attributable.
			if m, ok := r.Context().Value(requestMetaKey).(*requestMeta); ok {
				m.userId = userDb.Id
			}

			// Put userId into context
			ctx := auth.WithUser(r.Context(), userDb)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
