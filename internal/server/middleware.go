package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
)

type contextKey string

const requestIdKey contextKey = "requestId"

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
		requestId := generateRequestId()
		startTime := time.Now()

		logger := log.New(os.Stdout, "["+requestId+"]["+r.Method+":"+r.URL.Path+"] - ", log.LstdFlags)

		logger.Printf("Request received...")

		ctx := context.WithValue(r.Context(), requestIdKey, requestId)
		ctx = logx.WithLogger(ctx, logger)
		r = r.WithContext(ctx)

		recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(startTime)
		if duration > time.Second {
			logger.Printf("Request completed in %.2fs (status %d)", duration.Seconds(), recorder.statusCode)
		} else {
			logger.Printf("Request completed in %dms (status %d)", duration.Milliseconds(), recorder.statusCode)
		}
	})
}

////////////////////////////////////////////////////////////////////////////
//  AUTHENTICATION MIDDLEWARE
////////////////////////////////////////////////////////////////////////////

func AuthMiddleware(tokenSecret string, db *mongodb.DB) func(http.Handler) http.Handler {
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
			userId, err := auth.ValidateJWT(tokenString, tokenSecret)
			if err != nil {
				if _, ok := auth.ErrorsMap[err]; ok {
					api.RespondWithUnauthorized(w, err)
					return
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userDb, err := db.GetUserById(r.Context(), userId)
			if err == mongodb.ErrRecordNotFound || !userDb.IsActive {
				http.Error(w, "Invalid or inactive user", http.StatusUnauthorized)
				return
			}

			// Put userId into context
			ctx := auth.WithUser(r.Context(), userDb)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
