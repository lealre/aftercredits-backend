package server_test

import (
	"strings"
	"testing"

	"github.com/lealre/movies-backend/internal/server"
)

// NewServer must refuse to start when JWT_SECRET is unset (no insecure fallback).
func TestNewServer_RequiresJWTSecret(t *testing.T) {
	// imdbapi provider needs no API key, so the factory succeeds and we reach
	// the JWT_SECRET check without needing a DB connection.
	t.Setenv("TITLE_PROVIDER", "imdbapi")
	t.Setenv("JWT_SECRET", "")

	_, err := server.NewServer(nil)
	if err == nil {
		t.Fatal("expected NewServer to error when JWT_SECRET is unset")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected a JWT_SECRET error, got: %v", err)
	}
}
