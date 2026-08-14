package server

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/models"
)

type fakeSink struct {
	got  []models.ActivityEvent
	err  error
	call int
}

func (f *fakeSink) Append(_ context.Context, events []models.ActivityEvent) error {
	f.call++
	f.got = append(f.got, events...)
	return f.err
}

// handlerRecording returns a handler that records one event and responds with
// the given status, standing in for a real mutating endpoint.
func handlerRecording(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activity.Record(r.Context(), activity.TitleAdded("g1", "tt1", "Dune"))
		w.WriteHeader(status)
	})
}

func requestAs(method string, actor *models.User) *http.Request {
	r := httptest.NewRequest(method, "/anything", nil)
	if actor != nil {
		r = r.WithContext(auth.WithUser(r.Context(), *actor))
	}
	return r
}

func TestActivityMiddleware(t *testing.T) {
	actor := models.User{Id: "u1", Name: "Maria", Username: "maria"}

	t.Run("a successful mutating request flushes its events with the actor stamped", func(t *testing.T) {
		sink := &fakeSink{}
		ActivityMiddleware(sink)(handlerRecording(http.StatusCreated)).
			ServeHTTP(httptest.NewRecorder(), requestAs(http.MethodPost, &actor))

		require.Len(t, sink.got, 1)
		require.Equal(t, "u1", sink.got[0].ActorId)
		require.Equal(t, "Maria", sink.got[0].ActorName, "the display name comes from the authenticated user, not the emit site")
		require.Equal(t, "title_added", sink.got[0].Kind)
	})

	t.Run("a failed request records nothing", func(t *testing.T) {
		sink := &fakeSink{}
		ActivityMiddleware(sink)(handlerRecording(http.StatusConflict)).
			ServeHTTP(httptest.NewRecorder(), requestAs(http.MethodPost, &actor))

		require.Zero(t, sink.call, "the flush is gated on the response status")
	})

	t.Run("a failing sink does not change the response", func(t *testing.T) {
		sink := &fakeSink{err: errors.New("sink is down")}

		// A logger that writes into a buffer we can inspect, standing in for
		// the real one so the "logged, not propagated" half of the property
		// is asserted rather than only readable by eye in `-v` output.
		var logs bytes.Buffer
		ctx := logx.WithLogger(auth.WithUser(context.Background(), actor), log.New(&logs, "", 0))
		r := httptest.NewRequest(http.MethodPost, "/anything", nil).WithContext(ctx)

		w := httptest.NewRecorder()
		ActivityMiddleware(sink)(handlerRecording(http.StatusCreated)).
			ServeHTTP(w, r)

		require.Equal(t, http.StatusCreated, w.Code,
			"the write already committed; a lost feed line must not fail the user's request")
		require.Empty(t, w.Body.String(),
			"a lost feed line must not surface into the response body either")
		require.Contains(t, logs.String(), "sink is down",
			"the failure must at least be logged, since it is nowhere else")
	})

	t.Run("a read request is passed through untouched", func(t *testing.T) {
		sink := &fakeSink{}
		reached := false
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
			require.Empty(t, activity.Recorded(r.Context()), "no recorder is seeded for a read")
		})
		ActivityMiddleware(sink)(h).
			ServeHTTP(httptest.NewRecorder(), requestAs(http.MethodGet, &actor))

		require.True(t, reached)
		require.Zero(t, sink.call)
	})

	t.Run("events without an actor are dropped rather than written unattributed", func(t *testing.T) {
		sink := &fakeSink{}
		ActivityMiddleware(sink)(handlerRecording(http.StatusCreated)).
			ServeHTTP(httptest.NewRecorder(), requestAs(http.MethodPost, nil))

		require.Zero(t, sink.call)
	})
}
