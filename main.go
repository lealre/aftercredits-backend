package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/api"
)

func main() {
	godotenv.Load()

	err := ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", api.RootHandler)
	mux.HandleFunc("GET /titles", api.GetTitlesHandler)
	mux.HandleFunc("POST /titles", api.AddTitleHandler)
	mux.HandleFunc("PATCH /titles/{id}", api.SetWatched)
	mux.HandleFunc("DELETE /titles/{id}", api.DeleteTitle)
	mux.HandleFunc("GET /titles/{id}/ratings", api.GetTitleRatingsHandler)
	mux.HandleFunc("GET /ratings", api.GetAllRatingsHandler)
	mux.HandleFunc("GET /ratings/{id}", api.GetRatingByIdHandler)
	mux.HandleFunc("PATCH /ratings/{id}", api.UpdateRatingHandler)
	mux.HandleFunc("POST /ratings", api.AddRating)
	mux.HandleFunc("GET /users", api.GetUsers)
	wrappedMux := RequestIDMiddleware(mux)
	server := &http.Server{
		Addr:    ":8080",
		Handler: wrappedMux,
	}

	log.Println("Server is running on port 8080")
	return server.ListenAndServe()
}

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

		// Override the global logger for this request
		originalLogger := log.Default()
		defer func() {
			log.SetOutput(originalLogger.Writer()) // Restore original logger
		}()

		// Create a request-specific logger
		log.SetOutput(os.Stdout)
		log.SetPrefix("[" + requestID + "] - ")
		log.SetFlags(log.LstdFlags)

		// Log request start
		log.Printf("Started %s %s", r.Method, r.URL.Path)

		// Store requestID in context
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		// Wrap the response writer to capture status code
		recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(recorder, r)

		// Calculate duration
		duration := time.Since(startTime)

		// Log completion with status and duration
		if duration > time.Second {
			log.Printf("Completed %s %s in %.2fs (status %d)",
				r.Method, r.URL.Path, duration.Seconds(), recorder.statusCode)
		} else {
			log.Printf("Completed %s %s in %dms (status %d)",
				r.Method, r.URL.Path, duration.Milliseconds(), recorder.statusCode)
		}
	})
}
