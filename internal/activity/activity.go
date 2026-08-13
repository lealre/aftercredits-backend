// Package activity records what a request did, for the group activity feed.
//
// A write site calls Record(ctx, …) with one line: no store handle, no error
// path, no knowledge of transactions. The middleware that seeded the recorder
// persists the buffer into the request's unit-of-work transaction, and only
// when the response was a success — so "an event exists only if the change
// happened" is structural rather than something each call site remembers.
package activity

import (
	"context"
	"sync"
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

func TitleWatchedChanged(groupId, titleId, titleName string, watched bool, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{"watched": watched}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindTitleWatchedChanged, TitleId: tid, TitleName: tname, Payload: p}
}

func RatingAdded(groupId, titleId, titleName string, note float32, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{"note": float64(note)}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindRatingAdded, TitleId: tid, TitleName: tname, Payload: p}
}

func RatingUpdated(groupId, titleId, titleName string, note, previous float32, season *int) Event {
	tid, tname := title(titleId, titleName)
	p := map[string]any{"note": float64(note), "previousNote": float64(previous)}
	if season != nil {
		p["season"] = *season
	}
	return Event{GroupId: groupId, Kind: KindRatingUpdated, TitleId: tid, TitleName: tname, Payload: p}
}

func RatingDeleted(groupId, titleId, titleName string, previous float32) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindRatingDeleted, TitleId: tid, TitleName: tname,
		Payload: map[string]any{"previousNote": float64(previous)}}
}

func RatingSeasonDeleted(groupId, titleId, titleName string, season int, previous float32) Event {
	tid, tname := title(titleId, titleName)
	return Event{GroupId: groupId, Kind: KindRatingSeasonDeleted, TitleId: tid, TitleName: tname,
		Payload: map[string]any{"season": season, "previousNote": float64(previous)}}
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
