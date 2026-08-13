package activity

import (
	"context"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// Sink is where recorded events go once a request has succeeded. One method, so
// the destination is a swappable dependency: the store today, a broker or a
// fan-out to several later, with no change to the recorder, the eleven emit
// sites, or the read model.
//
// It takes models.ActivityEvent rather than Event: by flush time the actor has
// been resolved from the request, so what reaches a sink is a complete fact
// rather than the intent to record one. A sink knows about destinations, never
// about HTTP or auth.
type Sink interface {
	Append(ctx context.Context, events []models.ActivityEvent) error
}

// StoreSink is the phase 1 sink: it appends to activity_events through the
// store interface, so it inherits the database portability boundary the rest of
// the app already depends on.
type StoreSink struct{ store store.Store }

func NewStoreSink(st store.Store) StoreSink { return StoreSink{store: st} }

func (s StoreSink) Append(ctx context.Context, events []models.ActivityEvent) error {
	return s.store.InsertActivityEvents(ctx, events)
}
