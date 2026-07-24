package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A 429 with a Retry-After header should be retried after that delay, then succeed.
func TestGetTitle_RetriesOn429ThenSucceeds(t *testing.T) {
	var findCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/find/tt0068646", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&findCalls, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"movie_results":[{"id":238}],"tv_results":[]}`))
	})
	mux.HandleFunc("/movie/238", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":238,"title":"The Godfather","release_date":"1972-03-14","runtime":175}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	var slept []time.Duration
	p.sleep = func(d time.Duration) { slept = append(slept, d) } // capture instead of really sleeping

	got, err := p.GetTitle(context.Background(), "tt0068646")
	if err != nil {
		t.Fatalf("GetTitle after 429 retry: %v", err)
	}
	if got.ID != "tt0068646" || got.PrimaryTitle != "The Godfather" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if n := atomic.LoadInt32(&findCalls); n != 2 {
		t.Fatalf("expected 2 /find calls (1x 429 + 1 retry), got %d", n)
	}
	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("expected one Retry-After sleep of 2s, got %v", slept)
	}
}

// Persistent 429s should retry maxRetries times, then return a rate-limit error.
func TestGetJSON_429ExhaustsRetries(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/find/tt0000001", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests) // always 429, no Retry-After -> exponential backoff
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := newWithBaseURL(srv.URL, "key")
	p.sleep = func(time.Duration) {} // don't actually sleep
	p.maxRetries = 2

	_, err := p.GetTitle(context.Background(), "tt0000001")
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(strings.ToLower(err.Error()), "rate") {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 3 { // 1 initial attempt + 2 retries
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", n)
	}
}
