package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	activitycore "github.com/lealre/movies-backend/internal/activity"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// StreamTicket is what POST /activity/stream-ticket answers with. ExpiresIn is
// seconds, and is read from the ticket authority rather than restated here: the
// TTL is one fact, and a client that trusts a hard-coded 60 while the server
// expires at 30 would retry with a ticket that is already dead.
type StreamTicket struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expiresIn"`
}

// TicketAuthority mints and consumes the short-lived, single-use tickets that
// stand in for a Bearer token on the SSE endpoint (EventSource cannot send an
// Authorization header, and a JWT in the query string would leak into access
// logs and browser history).
//
// It is an interface for the same reason Sink is one: today it is the
// in-memory activity.TicketStore, and a deployment with more than one backend
// process would swap in a shared one without the service or the handler
// changing.
type TicketAuthority interface {
	Issue(userId string) string
	Redeem(ticket string) (userId string, ok bool)
	TTL() time.Duration
}

// Streamer owns the two process-wide handles the live stream needs: the
// fan-out hub every connection subscribes to, and the ticket authority that
// authenticates those connections. Both are built once per server, which is
// why this is a struct rather than the free functions the rest of this package
// uses — a per-call hub would fan out to nobody.
type Streamer struct {
	hub     *activitycore.Hub
	tickets TicketAuthority
}

func NewStreamer(hub *activitycore.Hub, tickets TicketAuthority) *Streamer {
	return &Streamer{hub: hub, tickets: tickets}
}

// IssueTicket mints a ticket for an already-authenticated user. The caller is
// responsible for that authentication: POST /activity/stream-ticket is
// deliberately NOT in api.PublicPaths, so a ticket can only ever be minted for
// the bearer of a valid token.
func (s *Streamer) IssueTicket(userId string) StreamTicket {
	return StreamTicket{
		Ticket:    s.tickets.Issue(userId),
		ExpiresIn: int(s.tickets.TTL().Seconds()),
	}
}

// OpenStream authenticates a stream connection by ticket and registers its
// subscriber with the hub. The caller must CloseStream the returned subscriber
// on every exit path or the hub keeps it — and its channel — forever.
//
// Unknown, expired and already-redeemed tickets all come back from Redeem as a
// plain false and all return ErrInvalidTicket, so the response cannot be used
// to tell a guessed ticket from a stale one.
//
// The membership read is GetUserById's, the same one AuthMiddleware performs
// for every other route and the same predicate the feed query uses: current
// members of non-deleted groups. It is resolved once, here, and captured in
// the subscription — a user who joins a group mid-stream sees it on their next
// connect, which is the trade the design doc makes to keep Publish
// query-free.
func (s *Streamer) OpenStream(db store.Store, ctx context.Context, ticket string) (*activitycore.Subscriber, error) {
	userId, ok := s.tickets.Redeem(ticket)
	if !ok {
		return nil, ErrInvalidTicket
	}

	// The error is checked before the boolean (CONVENTIONS §3): a store failure
	// leaves user as the zero value, whose IsActive is false, so folding the two
	// together would report a database outage as a bad ticket and send every
	// client into a reconnect loop that cannot succeed.
	user, err := db.GetUserById(ctx, userId)
	if err != nil && !errors.Is(err, store.ErrRecordNotFound) {
		return nil, err
	}
	// A ticket outliving its user by up to its TTL is possible; a deleted or
	// deactivated user must not get a stream out of it.
	if errors.Is(err, store.ErrRecordNotFound) || !user.IsActive {
		return nil, ErrInvalidTicket
	}

	return s.hub.Subscribe(user.Id, user.Groups), nil
}

// CloseStream removes a subscriber from the hub. It is safe to call more than
// once, so a handler can defer it without tracking whether it already ran.
func (s *Streamer) CloseStream(subscriber *activitycore.Subscriber) {
	s.hub.Unsubscribe(subscriber)
}

// StreamFrame renders one event as an SSE message.
//
// The data line is json.Marshal of MapDbEventToApiEvent — the same mapper and
// the same wire type GET /activity's events[] elements go through, so the two
// serializations of one fact cannot drift apart. That is the whole cost of
// pushing data instead of a signal, which is why there is one function here
// and not a second serializer, and why a test pins the two byte-identical.
//
// id: carries the event's seq. Nothing reads it back — reconnect takes a fresh
// snapshot rather than replaying from Last-Event-ID — but it costs nothing and
// leaves replay possible later.
func StreamFrame(event models.ActivityEvent) ([]byte, error) {
	data, err := json.Marshal(MapDbEventToApiEvent(event))
	if err != nil {
		return nil, err
	}
	return fmt.Appendf(nil, "id: %d\nevent: activity\ndata: %s\n\n", event.Seq, data), nil
}
