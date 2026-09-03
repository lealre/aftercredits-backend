package activity

import (
	"errors"
	"sync"

	"github.com/lealre/movies-backend/internal/models"
)

// Subscriber caps. maxSubscribersPerUser is per user id (a real browser opens a
// few tabs, so 1 would be wrong); maxSubscribersTotal bounds the whole process
// so no combination of accounts can exhaust memory/connections on the Pi.
const (
	maxSubscribersPerUser = 3
	maxSubscribersTotal   = 200
)

// ErrTooManySubscribers is returned by Subscribe when a cap is reached. The SSE
// handler maps it to 429.
var ErrTooManySubscribers = errors.New("too many open activity streams")

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
	perUser     map[string]int
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[*Subscriber]struct{}),
		perUser:     make(map[string]int),
	}
}

// Subscribe registers a new subscriber and returns its mailbox, unless a
// per-user or process-wide cap is reached (ErrTooManySubscribers). Callers must
// Unsubscribe when the connection ends, or the entry — and its channel — leaks
// for the life of the process.
func (h *Hub) Subscribe(userId string, groupIds []string) (*Subscriber, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.subscribers) >= maxSubscribersTotal {
		return nil, ErrTooManySubscribers
	}
	if h.perUser[userId] >= maxSubscribersPerUser {
		return nil, ErrTooManySubscribers
	}

	s := &Subscriber{
		UserId:   userId,
		GroupIds: append([]string(nil), groupIds...),
		Events:   make(chan models.ActivityEvent, subscriberBufferSize),
	}
	h.subscribers[s] = struct{}{}
	h.perUser[userId]++

	return s, nil
}

// Unsubscribe removes s from the hub and closes its channel. It is safe to
// call more than once — the SSE handler calls it from a defer and may also
// react to the channel closing on its own, and a second close would panic
// without the guard.
func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	if _, ok := h.subscribers[s]; ok {
		delete(h.subscribers, s)
		if h.perUser[s.UserId] <= 1 {
			delete(h.perUser, s.UserId)
		} else {
			h.perUser[s.UserId]--
		}
	}
	h.mu.Unlock()

	s.closeOnce.Do(func() { close(s.Events) })
}

// SubscriberCount reports how many subscribers are currently registered. It
// exists so a dropped connection can be shown to actually unsubscribe: a
// handler that forgets leaks one entry — and one channel — per lost client,
// which is invisible until the process runs out of memory.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.subscribers)
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
