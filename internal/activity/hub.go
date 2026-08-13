package activity

import (
	"sync"

	"github.com/lealre/movies-backend/internal/models"
)

// subscriberBufferSize is the per-subscriber channel capacity. It is small on
// purpose: the buffer only needs to absorb the gap between one event landing
// and the SSE handler's next write, not to queue a backlog for a stalled
// client. A stalled client is repaired by its next reconnect snapshot, not by
// a bigger buffer.
const subscriberBufferSize = 16

// Subscriber is one connected client's mailbox. GroupIds is captured once, at
// Subscribe time: a user who joins a group mid-stream starts seeing its
// activity on their next connect, not retroactively. Re-resolving membership
// on every Publish was rejected because it would turn every push into a
// query — see the phase 2 design doc.
type Subscriber struct {
	UserId   string
	GroupIds []string
	Events   chan models.ActivityEvent

	closeOnce sync.Once
}

// Hub is the in-process fan-out for activity events: one process-wide LISTEN
// loop publishes into it, and every SSE connection on this process holds a
// Subscriber registered with it.
//
// It deliberately knows nothing about Postgres, sqlc, or HTTP — it deals only
// in models.ActivityEvent and the Subscriber it hands back. The LISTEN loop
// (which does know about Postgres) and the SSE handler (which does know about
// HTTP) are the only callers.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[*Subscriber]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[*Subscriber]struct{})}
}

// Subscribe registers a new subscriber and returns its mailbox. Callers must
// Unsubscribe when the connection ends, or the entry — and its channel — leaks
// for the life of the process.
func (h *Hub) Subscribe(userId string, groupIds []string) *Subscriber {
	s := &Subscriber{
		UserId:   userId,
		GroupIds: append([]string(nil), groupIds...),
		Events:   make(chan models.ActivityEvent, subscriberBufferSize),
	}

	h.mu.Lock()
	h.subscribers[s] = struct{}{}
	h.mu.Unlock()

	return s
}

// Unsubscribe removes s from the hub and closes its channel. It is safe to
// call more than once — the SSE handler calls it from a defer and may also
// react to the channel closing on its own, and a second close would panic
// without the guard.
func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	delete(h.subscribers, s)
	h.mu.Unlock()

	s.closeOnce.Do(func() { close(s.Events) })
}

// Publish fans event out to every subscriber for whom it is visible, and never
// blocks: it runs on the single LISTEN goroutine that serves every connected
// client, so one slow or stalled subscriber must not delay delivery to
// anyone else. A send to a full channel is dropped rather than waited on —
// the client's next reconnect snapshot repairs a dropped frame, while a
// blocked Publish would stall every other subscriber for as long as the slow
// one stays full.
//
// The visibility predicate mirrors GetActivityFeedRows in
// sql/queries/activity.sql: the event's group must be one of the subscriber's
// groups, and the subscriber must not be the event's own actor. Keeping these
// in sync is deliberate — if they diverge, the stream shows the reader
// something the feed itself would not.
func (h *Hub) Publish(event models.ActivityEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for s := range h.subscribers {
		if !visible(event, s) {
			continue
		}
		select {
		case s.Events <- event:
		default:
			// Full buffer: drop rather than block. See the Publish doc comment.
		}
	}
}

func visible(event models.ActivityEvent, s *Subscriber) bool {
	if event.ActorId == s.UserId {
		return false
	}
	for _, g := range s.GroupIds {
		if g == event.GroupId {
			return true
		}
	}
	return false
}
