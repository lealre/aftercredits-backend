package activity

import (
	"context"

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

func MarkRead(db store.Store, ctx context.Context, userId string, seq int64) error {
	if seq <= 0 {
		return ErrInvalidSeq
	}
	return db.MarkActivityRead(ctx, userId, seq)
}
