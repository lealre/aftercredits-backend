package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
)

// InsertActivityEvents appends events to the log. It backs activity.StoreSink
// and is called after the request's business write has already committed, so it
// owns its own transaction: either the whole batch of events for a request
// lands or none of it does.
//
// Ids and created_at are generated here, matching every other write in this
// package. seq is assigned by the database.
func (s *Store) InsertActivityEvents(ctx context.Context, events []models.ActivityEvent) error {
	if len(events) == 0 {
		return nil
	}
	return s.inTx(ctx, func(q *database.Queries) error {
		now := time.Now()
		for _, e := range events {
			var payload []byte
			if e.Payload == nil {
				payload = []byte(`{}`)
			} else {
				var err error
				payload, err = json.Marshal(e.Payload)
				if err != nil {
					return err
				}
			}
			if _, err := q.InsertActivityEventRow(ctx, database.InsertActivityEventRowParams{
				ID:        firstNonEmpty(e.Id, uuid.NewString()),
				GroupID:   e.GroupId,
				ActorID:   e.ActorId,
				ActorName: e.ActorName,
				Kind:      e.Kind,
				TitleID:   ptrToText(e.TitleId),
				TitleName: ptrToText(e.TitleName),
				Payload:   payload,
				CreatedAt: timeToTimestamptz(now),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetActivityFeed returns the events visible to userId, newest first, excluding
// their own. before is an exclusive seq cursor; nil starts at the newest.
func (s *Store) GetActivityFeed(ctx context.Context, userId string, before *int64, limit int) ([]models.ActivityEvent, error) {
	if limit <= 0 {
		return []models.ActivityEvent{}, nil
	}
	rows, err := s.q.GetActivityFeedRows(ctx, database.GetActivityFeedRowsParams{
		UserID:   userId,
		Before:   int64PtrToNullable(before),
		RowLimit: int64(limit),
	})
	if err != nil {
		return []models.ActivityEvent{}, err
	}
	events := make([]models.ActivityEvent, 0, len(rows))
	for _, row := range rows {
		event, err := activityEventRowToModel(row)
		if err != nil {
			return []models.ActivityEvent{}, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *Store) GetActivityUnreadCount(ctx context.Context, userId string) (int64, error) {
	return s.q.CountActivityUnread(ctx, userId)
}

func (s *Store) MarkActivityRead(ctx context.Context, userId string, seq int64) error {
	return s.q.UpsertActivityRead(ctx, database.UpsertActivityReadParams{
		UserID:  userId,
		ReadAt:  timeToTimestamptz(time.Now()),
		ReadSeq: seq,
	})
}
