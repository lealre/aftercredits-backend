package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// listenChannel is the pg_notify channel InsertActivityEvents fires on (see
// activity.go and sql/queries/activity.sql's NotifyActivityEvent).
const listenChannel = "activity_events"

// listenMinBackoff and listenMaxBackoff bound the reconnect delay: fast
// enough that a blip recovers quickly, capped so a genuinely down database
// (the Pi restarting) doesn't get hammered with connection attempts.
const (
	listenMinBackoff = time.Second
	listenMaxBackoff = 30 * time.Second
)

// ListenActivity is the LISTEN loop backing real-time activity push: it
// LISTENs on listenChannel and turns every notified id into a row handed to
// publish.
//
// The connection is dialed standalone and never comes from the pool. LISTEN is
// connection-scoped, so a pooled connection handed back between the LISTEN and
// the wait loses the subscription silently — no error, just permanent silence.
// Worse, a subscribed connection idling in the pool keeps accumulating
// notifications in its socket buffer, which surface as stale events whenever it
// is handed out again. A connection dialed and closed outright cannot leak that
// way.
//
// publish is a plain func rather than *activity.Hub so this package never
// imports internal/activity, which must stay database-agnostic (CONVENTIONS §2).
//
// Connection errors log, back off (capped at listenMaxBackoff) and reconnect.
// Events landing in that gap are NOT replayed: clients repair themselves on
// their next snapshot read. A notified id whose row is gone is skipped, never
// fatal. Returns nil when ctx is cancelled, so a normal shutdown is not an
// error.
func (s *Store) ListenActivity(ctx context.Context, publish func(models.ActivityEvent)) error {
	// s.pool.Config() already returns a defensive copy (pgxpool.Pool.Config),
	// and pgx.ConnectConfig copies it again internally before each dial, so
	// reusing this same *pgx.ConnConfig across every reconnect attempt below
	// is safe — one parse of the pool's DSN-derived settings, not one per
	// attempt.
	connConfig := s.pool.Config().ConnConfig

	backoff := listenMinBackoff

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := s.listenOnce(ctx, connConfig, publish, func() { backoff = listenMinBackoff })
		if ctx.Err() != nil {
			return nil
		}

		slog.Warn("activity LISTEN connection lost, reconnecting", "err", err, "backoff", backoff.String())

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > listenMaxBackoff {
			backoff = listenMaxBackoff
		}
	}
}

// listenOnce dials one standalone connection, issues LISTEN, calls
// onConnected (which resets the caller's backoff — a connection that made it
// this far is evidence the outage, if any, is over), then blocks on
// WaitForNotification until ctx is cancelled or the connection errors.
//
// The connection is held for the entire call and Closed on the way out via
// the defer — never returned to any pool, so nothing else can ever be handed
// this exact session with a LISTEN still attached to it.
func (s *Store) listenOnce(ctx context.Context, connConfig *pgx.ConnConfig, publish func(models.ActivityEvent), onConnected func()) error {
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "LISTEN "+listenChannel); err != nil {
		return err
	}
	onConnected()

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			// Covers both ctx cancellation (the caller checks ctx.Err() and
			// treats it as a clean shutdown) and a genuine connection error
			// (the caller logs and reconnects).
			return err
		}

		event, err := s.GetActivityEventById(ctx, n.Payload)
		if err != nil {
			if errors.Is(err, store.ErrRecordNotFound) {
				slog.Warn("notified activity event has no matching row, skipping", "event_id", n.Payload)
				continue
			}
			slog.Error("failed to read notified activity event, skipping", "err", err, "event_id", n.Payload)
			continue
		}
		publish(event)
	}
}
