package activity

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ticketTTL is how long an issued ticket is redeemable. Short on purpose: a
// leaked ticket is worth one use and this window, never a standing
// credential. See the design doc's "Endpoints" section.
const ticketTTL = 60 * time.Second

// ticketBytes is the amount of crypto/rand entropy per ticket, hex-encoded to
// twice as many characters.
const ticketBytes = 32

type ticketEntry struct {
	userId    string
	expiresAt time.Time
}

// TicketStore issues and redeems single-use, short-lived tickets that stand in
// for a Bearer JWT on the SSE endpoint. EventSource cannot send an
// Authorization header, and putting the JWT in the query string would leak it
// into nginx access logs and browser history, so an authenticated
// POST /activity/stream-ticket mints a ticket here and GET /activity/stream
// redeems it exactly once.
//
// Deliberately in-memory: no schema migration, no new dependency, and losing
// the map on a process restart costs nothing worth persisting — a ticket is
// worth at most 60 seconds and one use.
//
// now is overridden by tests in this package to avoid time.Sleep(60s); it is
// unexported so the public constructor keeps the exact signature callers use.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
	now     func() time.Time
}

func NewTicketStore() *TicketStore {
	return &TicketStore{
		tickets: make(map[string]ticketEntry),
		now:     time.Now,
	}
}

// Issue mints a new single-use ticket for userId: 32 bytes from crypto/rand,
// hex-encoded to 64 characters. crypto/rand, never math/rand — a guessable
// ticket would let one user impersonate another on the stream.
//
// It also sweeps expired-but-unredeemed tickets from the map before inserting
// the new one, so a client that mints tickets without ever redeeming them
// cannot grow the map without bound between redemptions.
func (s *TicketStore) Issue(userId string) string {
	buf := make([]byte, ticketBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read only fails if the OS CSPRNG is unavailable, which
		// is unrecoverable for a function whose entire job is producing an
		// unguessable token.
		panic("activity: crypto/rand unavailable: " + err.Error())
	}
	ticket := hex.EncodeToString(buf)

	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked(now)
	s.tickets[ticket] = ticketEntry{userId: userId, expiresAt: now.Add(ticketTTL)}

	return ticket
}

// Redeem consumes ticket exactly once: a lookup and delete happen under the
// same lock, so of any number of goroutines redeeming the same ticket
// concurrently, exactly one observes it present and the rest see it already
// gone. An expired ticket is swept before the lookup, so it fails identically
// to one that was never issued.
func (s *TicketStore) Redeem(ticket string) (userId string, ok bool) {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked(now)

	entry, found := s.tickets[ticket]
	if !found {
		return "", false
	}
	delete(s.tickets, ticket)

	return entry.userId, true
}

// sweepLocked removes every entry that has reached its TTL. Callers must hold
// s.mu. Running it inline on Issue and Redeem — rather than on a background
// goroutine — is what keeps this store leak-free without anything to start,
// stop, or leak across the process's lifetime.
func (s *TicketStore) sweepLocked(now time.Time) {
	for t, e := range s.tickets {
		if !e.expiresAt.After(now) {
			delete(s.tickets, t)
		}
	}
}
