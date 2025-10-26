package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lealre/movies-backend/internal/logx"
)

// Define a custom type for context keys
type contextKey string

const (
	requestIDKey contextKey = "requestID"
)

// generateRequestID creates a unique 5-character identifier
func generateRequestID() string {
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

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()
		startTime := time.Now()

		// Create a request-specific logger (not global)
		logger := log.New(os.Stdout, "["+requestID+"]["+r.Method+":"+r.URL.Path+"] - ", log.LstdFlags)

		logger.Printf("Request received...")

		// Store logger and requestID in context
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
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
