package server

import (
	"context"
	"net/http"
	"time"

	"github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/models"
)

// flushTimeout bounds the post-response sink call. The request context is
// already done by then, so this is the only thing standing between a wedged
// sink and a goroutine parked forever.
const flushTimeout = 5 * time.Second

// ActivityMiddleware records what a request did, and hands it to the sink after
// the request has succeeded — never inside the business transaction.
//
// Three properties, in the order they matter:
//
//   - A failing sink cannot fail the user's request. The write has already
//     committed and the response has already gone out; a lost feed line is
//     logged and swallowed. This is the whole reason the atomic design was
//     dropped (see the spec), so it is asserted by a test, not just intended.
//   - Nothing is recorded for a request that did not succeed. The flush is
//     gated on the response status, so a rejected or failed request leaves no
//     event behind. What is deliberately NOT promised is the converse: a
//     handler that commits and then fails leaves a row with no event, which is
//     what best-effort delivery means.
//   - The actor is stamped centrally. It runs INSIDE AuthMiddleware, so the
//     authenticated user is in the context and no emit site has to pass it.
//
// Read requests are passed straight through — they have nothing to record.
func ActivityMiddleware(sink activity.Sink) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutating(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := activity.WithRecorder(r.Context())
			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.statusCode >= http.StatusBadRequest {
				return
			}
			flush(ctx, sink, activity.Recorded(ctx))
		})
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// flush stamps the actor onto each buffered event and appends them to the sink.
// It never returns an error: the response is already written, so the only
// honest thing left to do with a failure is log it.
//
// The context is detached from the request. The client may already have
// disconnected — cancelling the flush for that reason would drop events for the
// people the feed is actually for.
func flush(ctx context.Context, sink activity.Sink, events []activity.Event) {
	if len(events) == 0 {
		return
	}

	logger := logx.FromContext(ctx)

	actor := auth.GetUserFromContext(ctx)
	if actor == nil {
		// Only the two public routes have no actor, and neither records
		// anything. An unattributable row would be worse than no row.
		logger.Printf("WARN: %d activity event(s) dropped: no actor in context", len(events))
		return
	}

	stamped := make([]models.ActivityEvent, 0, len(events))
	for _, e := range events {
		stamped = append(stamped, toModel(*actor, e))
	}

	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), flushTimeout)
	defer cancel()

	if err := sink.Append(flushCtx, stamped); err != nil {
		// Deliberately not propagated: the business write is committed and the
		// response is sent. Best-effort delivery, stated in the spec.
		logger.Printf("ERROR: recording %d activity event(s) failed: %v", len(stamped), err)
	}
}

func toModel(actor models.User, e activity.Event) models.ActivityEvent {
	return models.ActivityEvent{
		GroupId:   e.GroupId,
		ActorId:   actor.Id,
		ActorName: actorDisplayName(actor),
		Kind:      e.Kind,
		TitleId:   e.TitleId,
		TitleName: e.TitleName,
		Payload:   e.Payload,
	}
}

// actorDisplayName prefers the user's name and falls back to the username, so a
// feed line always has something to call the actor.
func actorDisplayName(u models.User) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}
