package activity

import (
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixedClock returns a clock function pinned to t, so tests control time
// without time.Sleep(60s).
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestTicketStore(t *testing.T) {
	t.Run("issue then redeem returns the user", func(t *testing.T) {
		s := NewTicketStore()
		ticket := s.Issue("alice")

		userId, ok := s.Redeem(ticket)
		require.True(t, ok, "a freshly issued ticket should redeem successfully")
		require.Equal(t, "alice", userId)
	})

	t.Run("a second redemption of the same ticket fails", func(t *testing.T) {
		s := NewTicketStore()
		ticket := s.Issue("alice")

		_, ok := s.Redeem(ticket)
		require.True(t, ok, "first redemption should succeed")

		_, ok = s.Redeem(ticket)
		require.False(t, ok, "a redeemed ticket must not be redeemable again")
	})

	t.Run("an expired ticket fails even if never used", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		s := NewTicketStore()
		s.now = fixedClock(start)

		ticket := s.Issue("alice")

		// Advance the clock past the TTL without ever redeeming.
		s.now = fixedClock(start.Add(ticketTTL + time.Second))

		_, ok := s.Redeem(ticket)
		require.False(t, ok, "a ticket past its TTL must fail to redeem")
	})

	t.Run("a ticket redeemed exactly at expiry fails", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		s := NewTicketStore()
		s.now = fixedClock(start)

		ticket := s.Issue("alice")

		s.now = fixedClock(start.Add(ticketTTL))

		_, ok := s.Redeem(ticket)
		require.False(t, ok, "TTL is exclusive: a ticket redeemed exactly at expiresAt must fail")
	})

	t.Run("a ticket redeemed just under its TTL still succeeds", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		s := NewTicketStore()
		s.now = fixedClock(start)

		ticket := s.Issue("alice")

		s.now = fixedClock(start.Add(ticketTTL - time.Millisecond))

		_, ok := s.Redeem(ticket)
		require.True(t, ok, "a ticket redeemed before its TTL elapses must still work")
	})

	t.Run("an unknown ticket fails", func(t *testing.T) {
		s := NewTicketStore()

		_, ok := s.Redeem("does-not-exist")
		require.False(t, ok, "redeeming a ticket that was never issued must fail")
	})

	t.Run("tickets are unguessable: distinct and 64 hex characters", func(t *testing.T) {
		s := NewTicketStore()

		a := s.Issue("alice")
		b := s.Issue("alice")

		require.NotEqual(t, a, b, "two issues for the same user must not collide")
		require.Len(t, a, 64, "32 bytes hex-encoded is 64 characters")
		require.Len(t, b, 64, "32 bytes hex-encoded is 64 characters")

		_, err := hex.DecodeString(a)
		require.NoError(t, err, "ticket must be valid hex")
	})

	t.Run("expired unredeemed tickets are swept and do not grow the map forever", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		s := NewTicketStore()
		s.now = fixedClock(start)

		for range 5 {
			s.Issue("alice")
		}
		require.Len(t, s.tickets, 5, "all five tickets should be present before expiry")

		// Advance past the TTL and issue one more: both Issue and Redeem sweep,
		// so the five stale entries must be gone, leaving only the new one.
		s.now = fixedClock(start.Add(ticketTTL + time.Second))
		s.Issue("bob")

		require.Len(t, s.tickets, 1, "expired entries must be swept on Issue, not accumulate forever")
	})

	t.Run("expired unredeemed tickets are swept on Redeem too", func(t *testing.T) {
		start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		s := NewTicketStore()
		s.now = fixedClock(start)

		stale := s.Issue("alice")
		s.Issue("carol")

		s.now = fixedClock(start.Add(ticketTTL + time.Second))

		_, ok := s.Redeem(stale)
		require.False(t, ok, "the stale ticket itself must fail")
		require.Empty(t, s.tickets, "Redeem must sweep every expired entry, not just the one looked up")
	})

	t.Run("concurrent redemption of the same ticket: exactly one caller wins", func(t *testing.T) {
		s := NewTicketStore()
		ticket := s.Issue("alice")

		const attempts = 50
		var wg sync.WaitGroup
		var successes int
		var mu sync.Mutex

		for range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, ok := s.Redeem(ticket); ok {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		require.Equal(t, 1, successes, "exactly one concurrent redemption of the same ticket must succeed")
	})

	t.Run("concurrent issue and redeem across many users", func(t *testing.T) {
		s := NewTicketStore()

		const users = 20
		tickets := make([]string, users)
		for i := range users {
			tickets[i] = s.Issue("user")
		}

		var wg sync.WaitGroup
		results := make([]bool, users)
		for i := range users {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, ok := s.Redeem(tickets[i])
				results[i] = ok
			}(i)
		}
		wg.Wait()

		for i, ok := range results {
			require.True(t, ok, "distinct tickets redeemed concurrently should each succeed once, ticket index %d", i)
		}
	})
}
