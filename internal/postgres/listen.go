package postgres

import (
	"context"
	"errors"
	"log"
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
// dials its own standalone connection (never the shared pool), LISTENs on
// listenChannel, and turns every notified id into a row (via
// GetActivityEventById) handed to publish.
//
// The connection is dedicated on purpose: LISTEN is connection-scoped state,
// and a pooled connection that got handed back to the pool between the
// LISTEN and the wait would lose the subscription with no error, just
// permanent silence. Going through pgx.ConnectConfig (rather than
// s.pool.Acquire, held for the loop's lifetime) sidesteps a subtler version
// of the same problem: a connection released to the shared pool after a
// LISTEN stays subscribed at the Postgres session level even while idle in
// the pool, so it keeps silently accumulating notifications in its unread
// socket buffer; if that exact connection is ever handed out again — to this
// loop's own reconnect, or to an unrelated query elsewhere in the app — that
// backlog surfaces as a stale notification with no connection to anything
// currently happening. A connection dialed and Closed outright, never
// touching the pool, cannot leak that way.
//
// publish takes a plain func rather than *activity.Hub so this package never
// imports internal/activity: internal/postgres names a concrete database,
// and the hub must stay database-agnostic (CONVENTIONS §2). The caller
// (server.go) wires ListenActivity's publish to hub.Publish.
//
// On any connection error — the dedicated connection dying, the Postgres
// backend restarting — this logs, backs off (capped at listenMaxBackoff),
// and reconnects, re-issuing LISTEN. Events that land during that gap are not
// replayed: nothing here tracks a resume position, and the design deliberately
// leans on clients repairing themselves via their next snapshot read (see
// "Snapshot, then stream" in the phase 2 design doc) rather than the backend
// trying to guarantee delivery.
//
// A notified id whose row is gone by the time it's read (deleted between the
// NOTIFY and this loop's read — not possible for activity_events today, since
// nothing deletes rows, but the loop doesn't assume that) is logged and
// skipped, never fatal.
//
// Returns nil when ctx is cancelled, so a normal shutdown is not an error.
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

		log.Printf("activity: LISTEN connection lost, reconnecting in %s: %v", backoff, err)

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
				log.Printf("activity: notified event %q has no matching row, skipping", n.Payload)
				continue
			}
			log.Printf("activity: failed to read notified event %q, skipping: %v", n.Payload, err)
			continue
		}
		publish(event)
	}
}
