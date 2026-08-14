package activity

import (
	"context"
	"errors"

	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/store"
)

// GetFeed returns the caller's feed newest-first. limit is normalized here, not
// in the store, matching how every other paged read in this codebase splits
// policy from storage.
func GetFeed(db store.Store, ctx context.Context, userId string, before *int64, limit int) (Feed, error) {
	limit, _ = config.NormalizePageParams(limit, 1)

	// One extra row answers "is there another page" without a second query.
	rows, err := db.GetActivityFeed(ctx, userId, before, limit+1)
	if err != nil {
		return Feed{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, MapDbEventToApiEvent(row))
	}

	feed := Feed{Events: events, HasMore: hasMore}
	if hasMore && len(events) > 0 {
		last := events[len(events)-1].Seq
		feed.NextBefore = &last
	}
	return feed, nil
}

func GetUnreadCount(db store.Store, ctx context.Context, userId string) (UnreadCount, error) {
	n, err := db.GetActivityUnreadCount(ctx, userId)
	if err != nil {
		return UnreadCount{}, err
	}
	return UnreadCount{Unread: n}, nil
}

// MarkEventRead marks exactly one event read for one user. Clicking a row twice
// is a success, not an error: the store's write is idempotent, so the second
// call changes nothing and the badge does not move twice.
//
// An event the caller cannot see is ErrEventNotFound, whether it is another
// group's, their own, or an id that never existed — the store collapses the
// three, and so does the answer.
func MarkEventRead(db store.Store, ctx context.Context, userId, eventId string) error {
	if eventId == "" {
		return ErrEventNotFound
	}
	if err := db.MarkActivityEventRead(ctx, userId, eventId); err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrEventNotFound
		}
		return err
	}
	return nil
}

// MarkAllRead clears the caller's badge: every event visible to them right now
// becomes read. Events recorded afterwards are unread, which is what makes the
// badge rise again.
func MarkAllRead(db store.Store, ctx context.Context, userId string) error {
	return db.MarkAllActivityEventsRead(ctx, userId)
}
