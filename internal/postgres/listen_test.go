package postgres

import (
	"bytes"
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
)

// listenTestTimeout bounds every receive in this file: a listener test that
// waits forever hangs the whole suite instead of failing it.
const listenTestTimeout = 5 * time.Second

// publishRecorder collects events handed to ListenActivity's publish
// callback and lets a test wait for the next one without a race on a plain
// slice (the loop runs on its own goroutine while the test reads).
type publishRecorder struct {
	mu     sync.Mutex
	events []models.ActivityEvent
	notify chan struct{}
}

func newPublishRecorder() *publishRecorder {
	return &publishRecorder{notify: make(chan struct{}, 1)}
}

func (r *publishRecorder) publish(e models.ActivityEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

// waitForCount blocks until at least n events have been recorded, or fails
// the test once timeout elapses.
func (r *publishRecorder) waitForCount(t *testing.T, n int, timeout time.Duration) []models.ActivityEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		r.mu.Lock()
		got := len(r.events)
		snapshot := append([]models.ActivityEvent(nil), r.events...)
		r.mu.Unlock()

		if got >= n {
			return snapshot
		}
		select {
		case <-r.notify:
		case <-deadline:
			t.Fatalf("timed out after %s waiting for %d published event(s), got %d", timeout, n, got)
		}
	}
}

// countListeners returns how many sessions currently show the exact LISTEN
// command as their last query in pg_stat_activity.
func countListeners(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE query = 'LISTEN '||$1`, listenChannel).Scan(&count)
	require.NoError(t, err, "failed to query pg_stat_activity")
	return count
}

// waitForListenerReady polls pg_stat_activity until a session beyond
// baseline is registered with the LISTEN query, proving ListenActivity's
// dedicated connection has issued LISTEN and is now blocked in
// WaitForNotification.
//
// This closes the inherent LISTEN/NOTIFY race: a NOTIFY fired before a
// session issues LISTEN is delivered to nobody and never queued, so a test
// that fires a NOTIFY (via InsertActivityEvents or directly) right after
// spawning the loop's goroutine — with no guarantee that goroutine has run
// yet, let alone finished acquiring a connection and issuing LISTEN — would
// flake under load. baseline is measured before the goroutine starts, so a
// stale registration left behind by an earlier subtest's connection (which
// stays registered after Release, since LISTEN is undone by connection
// close, not by returning to the pool) doesn't produce a false positive.
func waitForListenerReady(t *testing.T, pool *pgxpool.Pool, baseline int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if countListeners(t, pool) > baseline {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for the LISTEN loop to register with postgres", timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// listenerPids returns the backend pids of every session currently showing
// the exact LISTEN command as their last query — i.e. every dedicated
// connection presently blocked in WaitForNotification.
func listenerPids(t *testing.T, pool *pgxpool.Pool) map[int32]bool {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT pid FROM pg_stat_activity WHERE query = 'LISTEN '||$1`, listenChannel)
	require.NoError(t, err, "failed to query pg_stat_activity")
	defer rows.Close()

	pids := map[int32]bool{}
	for rows.Next() {
		var pid int32
		require.NoError(t, rows.Scan(&pid))
		pids[pid] = true
	}
	require.NoError(t, rows.Err())
	return pids
}

// waitForNewListenerPid polls until a pid appears in listenerPids that is in
// neither exclude (pids known before this wait started) nor already seen —
// i.e. a genuinely new dedicated connection has issued LISTEN.
func waitForNewListenerPid(t *testing.T, pool *pgxpool.Pool, exclude map[int32]bool, timeout time.Duration) int32 {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for pid := range listenerPids(t, pool) {
			if !exclude[pid] {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for a new LISTEN connection", timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// syncBuffer is a concurrency-safe io.Writer/String() pair, used to capture
// log.Printf output from the listener's background goroutine while the test
// goroutine reads it — log's own mutex serializes the Write calls, but
// reading the buffer from a different goroutine still needs its own lock.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startListening spawns ListenActivity in a goroutine against a cancellable
// context, registers cleanup to cancel it and assert it returns, and blocks
// until its dedicated connection has actually issued LISTEN (via
// waitForListenerReady) before returning — so callers can insert/notify
// immediately afterward without racing the loop's own startup.
func startListening(t *testing.T, s *Store, publish func(models.ActivityEvent)) context.Context {
	t.Helper()

	pool := newTestPool(t)
	baseline := countListeners(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenActivity(ctx, publish) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err, "a cancelled context must not be reported as an error")
		case <-time.After(listenTestTimeout):
			t.Fatal("ListenActivity did not return after ctx cancellation")
		}
	})

	waitForListenerReady(t, pool, baseline, listenTestTimeout)
	return ctx
}

func TestStore_ListenActivity(t *testing.T) {
	t.Run("an inserted event arrives at publish with the right kind and group name", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)

		rec := newPublishRecorder()
		ctx := startListening(t, s, rec.publish)

		actor := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "listen", actor))
		require.NoError(t, err, "failed to seed the group")

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
			GroupId: group.Id, ActorId: actor, ActorName: "Maria", Kind: "rating_added",
		}}))

		events := rec.waitForCount(t, 1, listenTestTimeout)
		require.Len(t, events, 1, "expected exactly one published event")
		require.Equal(t, "rating_added", events[0].Kind)
		require.Equal(t, group.Name, events[0].GroupName)
		require.Equal(t, "Maria", events[0].ActorName)
	})

	t.Run("a notified id with no matching row is skipped, the loop keeps running", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)

		rec := newPublishRecorder()
		ctx := startListening(t, s, rec.publish)

		_, err := newTestPool(t).Exec(ctx, "SELECT pg_notify($1, 'no-such-event-id')", listenChannel)
		require.NoError(t, err, "failed to fire the bogus notification")

		actor := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "listen-skip", actor))
		require.NoError(t, err, "failed to seed the group")

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
			GroupId: group.Id, ActorId: actor, ActorName: "Diego", Kind: "title_added",
		}}))

		events := rec.waitForCount(t, 1, listenTestTimeout)
		require.Len(t, events, 1, "the bogus id must be skipped, not published, and must not kill the loop")
		require.Equal(t, "title_added", events[0].Kind)
		require.Equal(t, group.Name, events[0].GroupName)
		require.Equal(t, "Diego", events[0].ActorName)
	})

	t.Run("reconnects with backoff after the connection is lost, and keeps delivering", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		pool := newTestPool(t)

		// Capture log output for the duration of this subtest, so the
		// "reconnecting" log line the brief requires can be asserted on,
		// without changing ListenActivity's signature to accept a logger.
		var logBuf syncBuffer
		prevOutput := log.Writer()
		prevFlags := log.Flags()
		log.SetOutput(&logBuf)
		t.Cleanup(func() {
			log.SetOutput(prevOutput)
			log.SetFlags(prevFlags)
		})

		rec := newPublishRecorder()
		before := listenerPids(t, pool)
		ctx := startListening(t, s, rec.publish)

		firstPid := waitForNewListenerPid(t, pool, before, listenTestTimeout)

		actor := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "reconnect", actor))
		require.NoError(t, err, "failed to seed the group")

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
			GroupId: group.Id, ActorId: actor, ActorName: "Before", Kind: "title_added",
		}}))
		rec.waitForCount(t, 1, listenTestTimeout)

		// Simulate the Pi's Postgres restarting out from under the loop: kill
		// the dedicated connection's backend directly.
		_, err = pool.Exec(ctx, "SELECT pg_terminate_backend($1)", firstPid)
		require.NoError(t, err, "failed to terminate the listener's backend")

		// A new, different pid taking over the LISTEN registration is the
		// proof of reconnect — the killed one cannot resurrect itself.
		excludeAfterKill := map[int32]bool{firstPid: true}
		for pid := range before {
			excludeAfterKill[pid] = true
		}
		secondPid := waitForNewListenerPid(t, pool, excludeAfterKill, listenTestTimeout)
		require.NotEqual(t, firstPid, secondPid, "the loop must reconnect on a new connection")

		require.Contains(t, logBuf.String(), "reconnecting",
			"the reconnect must be logged so an operator can see it happened")

		// The loop must be fully functional again: a fresh insert still
		// reaches publish.
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
			GroupId: group.Id, ActorId: actor, ActorName: "After", Kind: "title_removed",
		}}))
		events := rec.waitForCount(t, 2, listenTestTimeout)
		require.Equal(t, "After", events[1].ActorName, "the event after reconnect must still be delivered")
	})

	t.Run("returns nil when ctx is cancelled", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- s.ListenActivity(ctx, func(models.ActivityEvent) {}) }()

		cancel()
		select {
		case err := <-done:
			require.NoError(t, err, "a cancelled context must not be reported as an error")
		case <-time.After(listenTestTimeout):
			t.Fatal("ListenActivity did not return after ctx cancellation")
		}
	})
}
