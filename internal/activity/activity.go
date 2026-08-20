// Package activity records what a request did, for the group activity feed.
//
// A write site calls Record(ctx, …) with one line: no store handle, no error
// path, no knowledge of transactions. Record buffers the event in a
// per-request recorder that the middleware seeds; after the handler returns,
// and only if the response was not an error, the middleware stamps the actor
// onto each buffered event and hands them to a Sink.
//
// Delivery is best-effort, not atomic with the business write: the flush
// happens after the write has already committed, on its own context detached
// from the request. So a Record call is not a promise that the event will be
// stored — the process can die, or the sink can be down, between commit and
// flush (see the spec's "Delivery guarantee").
//
// Record on a context with no recorder is a silent no-op — no panic, nothing
// buffered. That is also how the feature turns off: when the flag is
// disabled the middleware is never installed, no recorder is ever seeded, and
// every one of the eleven call sites becomes a no-op without any of them
// having to know it.
package activity

import (
	"context"
	"math"
	"sync"
	"time"
)

const (
	KindTitleAdded           = "title_added"
	KindTitleRemoved         = "title_removed"
	KindTitleWatchedChanged  = "title_watched_changed"
	KindRatingAdded          = "rating_added"
	KindRatingUpdated        = "rating_updated"
	KindRatingDeleted        = "rating_deleted"
	KindRatingSeasonDeleted  = "rating_season_deleted"
	KindCommentAdded         = "comment_added"
	KindCommentUpdated       = "comment_updated"
	KindCommentDeleted       = "comment_deleted"
	KindCommentSeasonDeleted = "comment_season_deleted"
)

// Event is what happened, minus who and when: the actor and the timestamp are
// stamped centrally at flush time from the request's authenticated user.
type Event struct {
	GroupId   string
	Kind      string
	TitleId   *string
	TitleName *string
	Payload   map[string]any
}

type ctxKey struct{}

type recorder struct {
	mu     sync.Mutex
	events []Event
}

func WithRecorder(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, &recorder{})
}

// Record buffers an event. It is deliberately silent when no recorder is
// present: store and service code is also exercised by tests and the routines
// binary, and neither should have to care.
func Record(ctx context.Context, e Event) {
	r, _ := ctx.Value(ctxKey{}).(*recorder)
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func Recorded(ctx context.Context) []Event {
	r, _ := ctx.Value(ctxKey{}).(*recorder)
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

func title(id, name string) (*string, *string) { return &id, &name }

func TitleAdded(groupId, titleId, titleName string) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindTitleAdded, TitleId: tid, TitleName: tname}
}

func TitleRemoved(groupId, titleId, titleName string) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindTitleRemoved, TitleId: tid, TitleName: tname}
}

// WatchedState is one side of a watched change: the flag, and the date it was
// watched when there is one. It is scoped to whatever the request addressed —
// the title as a whole for a movie, a single season for a series.
type WatchedState struct {
	Watched   bool
	WatchedAt *time.Time
}

// TitleWatchedChanged carries both sides of the change, not just the result,
// because one kind has to cover several distinct sentences and only the
// before/after pair separates them: marked watched, marked not watched, a date
// added where there was none, a date changed from one day to another. A payload
// of {watched} alone made "marked as watched" and "moved the date" identical on
// the wire, which is the defect this fixes.
//
// previousNote on RatingUpdated is the same idea; previousWatched and
// previousWatchedAt follow its naming.
//
// The two dates are only present when they exist, so the frontend can test for
// presence rather than for a zero time: no watchedAt means the resulting state
// carries no date, no previousWatchedAt means there was none before.
func TitleWatchedChanged(groupId, titleId, titleName string, current, previous WatchedState, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{
		"watched":         current.Watched,
		"previousWatched": previous.Watched,
	}
	if current.WatchedAt != nil {
		p["watchedAt"] = *current.WatchedAt
	}
	if previous.WatchedAt != nil {
		p["previousWatchedAt"] = *previous.WatchedAt
	}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindTitleWatchedChanged, TitleId: tid, TitleName: tname, Payload: p}
}

// noteValue is the only place a stored note becomes a payload number.
//
// Notes are one-decimal values by contract, but they are stored in a REAL
// column — a float32, which cannot represent 5.6. Widening that straight to
// float64 with float64(note) resurrects the full binary expansion, and
// encoding/json then writes every digit of it: a feed line read "rated 1917
// with note 5.599999904632568". Marshalling the float32 directly would print
// "5.6", but the payload is map[string]any and any float32 in it would be
// widened by the JSON encoder anyway, so the rounding has to happen here.
//
// This treats the symptom. The cause is the column type, and rounding here
// only holds because the contract is one decimal — if that ever changes, this
// changes with it.
func noteValue(note float32) float64 {
	return math.Round(float64(note)*10) / 10
}

func RatingAdded(groupId, titleId, titleName string, note float32, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{"note": noteValue(note)}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindRatingAdded, TitleId: tid, TitleName: tname, Payload: p}
}

func RatingUpdated(groupId, titleId, titleName string, note, previous float32, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{"note": noteValue(note), "previousNote": noteValue(previous)}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindRatingUpdated, TitleId: tid, TitleName: tname, Payload: p}
}

func RatingDeleted(groupId, titleId, titleName string, previous float32) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindRatingDeleted, TitleId: tid, TitleName: tname,
		Payload: map[string]any{"previousNote": noteValue(previous)}}
}

func RatingSeasonDeleted(groupId, titleId, titleName string, season int, previous float32) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindRatingSeasonDeleted, TitleId: tid, TitleName: tname,
		Payload: map[string]any{"season": season, "previousNote": noteValue(previous)}}
}

func CommentAdded(groupId, titleId, titleName string, season *int) Event {
	return commentEvent(KindCommentAdded, groupId, titleId, titleName, season)
}

func CommentUpdated(groupId, titleId, titleName string, season *int) Event {
	return commentEvent(KindCommentUpdated, groupId, titleId, titleName, season)
}

func CommentDeleted(groupId, titleId, titleName string) Event {
	return commentEvent(KindCommentDeleted, groupId, titleId, titleName, nil)
}

func CommentSeasonDeleted(groupId, titleId, titleName string, season int) Event {
	return commentEvent(KindCommentSeasonDeleted, groupId, titleId, titleName, &season)
}

// commentEvent is shared by the four comment kinds. Comment bodies are
// deliberately not copied into the payload: they can be long, they can be
// edited afterwards, and the feed links to the title where the current text is.
func commentEvent(kind, groupId, titleId, titleName string, season *int) Event {
	tid, tname := title(titleId, titleName)
	var p map[string]any
	if season != nil {
		p = map[string]any{"season": *season}
	}
	return Event{GroupId: groupId, Kind: kind, TitleId: tid, TitleName: tname, Payload: p}
}
