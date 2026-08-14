package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// readStateById reads userId's whole feed and returns each event's read flag by
// event id. Per-row read state is only meaningful as a set — "e3 is read and
// nothing else moved" — so the assertions compare whole maps rather than
// picking at one row and hoping about the rest.
func readStateById(t *testing.T, s *Store, userId string) map[string]bool {
	t.Helper()

	feed, err := s.GetActivityFeed(context.Background(), userId, nil, 100)
	require.NoError(t, err, "failed to read the feed")

	state := map[string]bool{}
	for _, event := range feed {
		state[event.Id] = event.Read
	}
	return state
}

func TestStore_ActivityEvents(t *testing.T) {
	t.Run("a reader sees other members' events but never their own", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "feed", actor))
		require.NoError(t, err, "failed to seed the group")
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "actor", Kind: "rating_added"},
			{Id: "e2", GroupId: group.Id, ActorId: reader, ActorName: "reader", Kind: "rating_added"},
		}), "failed to insert the events")

		feed, err := s.GetActivityFeed(ctx, reader, nil, 50)
		require.NoError(t, err)
		require.Len(t, feed, 1, "the reader must see exactly the actor's event, not their own")
		require.Equal(t, "e1", feed[0].Id)
		require.NotZero(t, feed[0].Seq, "seq must be assigned by the database")
	})

	t.Run("a non-member sees nothing from the group", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		outsider := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "private", actor))
		require.NoError(t, err)
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "actor", Kind: "title_added"},
		}))

		feed, err := s.GetActivityFeed(ctx, outsider, nil, 50)
		require.NoError(t, err)
		require.Empty(t, feed, "a non-member must see none of the group's events")
		require.NotNil(t, feed, "read-many must return an empty slice, never nil")
	})

	t.Run("a departed member loses the group's history immediately", func(t *testing.T) {
		// Membership is joined at read time (sql/queries/activity.sql's own
		// doc comment claims this): a member who leaves must lose visibility
		// into the group's feed and badge on their very next read, with no
		// separate cleanup step.
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "departed", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "actor", Kind: "title_added"},
		}))

		feed, err := s.GetActivityFeed(ctx, reader, nil, 50)
		require.NoError(t, err)
		require.Len(t, feed, 1, "while still a member, the reader must see the actor's event")

		require.NoError(t, s.RemoveUserFromGroup(ctx, group.Id, reader))

		feed, err = s.GetActivityFeed(ctx, reader, nil, 50)
		require.NoError(t, err)
		require.Empty(t, feed, "a departed member must lose the group's history immediately")
		require.NotNil(t, feed, "read-many must return an empty slice, never nil")

		n, err := s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.Zero(t, n, "a departed member's badge must not count events from a group they left")
	})

	t.Run("marking one event read leaves every other event alone", func(t *testing.T) {
		// The property the watermark could not express: read state is per row,
		// so marking the middle event read must not touch the two older or the
		// two newer ones, and must move the badge by exactly one.
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "unread", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
			{Id: "e2", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "rating_added"},
			{Id: "e3", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "comment_added"},
			{Id: "e4", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "rating_updated"},
			{Id: "e5", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_removed"},
		}))

		n, err := s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.EqualValues(t, 5, n, "all five of the actor's events start unread")

		actorUnread, err := s.GetActivityUnreadCount(ctx, actor)
		require.NoError(t, err)
		require.Zero(t, actorUnread, "an actor's own events must never count toward their own badge")

		require.NoError(t, s.MarkActivityEventRead(ctx, reader, "e3"))

		n, err = s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.EqualValues(t, 4, n, "marking one event read drops the badge by exactly one")

		require.Equal(t, map[string]bool{"e1": false, "e2": false, "e3": true, "e4": false, "e5": false},
			readStateById(t, s, reader),
			"only e3 is read — the older events below it and the newer ones above it are untouched")
	})

	t.Run("marking the same event read twice changes nothing", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "idempotent", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
			{Id: "e2", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "rating_added"},
		}))

		require.NoError(t, s.MarkActivityEventRead(ctx, reader, "e1"))
		require.NoError(t, s.MarkActivityEventRead(ctx, reader, "e1"),
			"marking an already-read event read must succeed, not error")

		n, err := s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.EqualValues(t, 1, n, "the second mark must not double-count against the badge")
	})

	t.Run("an event the reader cannot see cannot be marked read", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		outsider := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "private", actor))
		require.NoError(t, err)
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
		}))

		require.ErrorIs(t, s.MarkActivityEventRead(ctx, outsider, "e1"), store.ErrRecordNotFound,
			"a non-member must not be able to mark a group's event read")
		require.ErrorIs(t, s.MarkActivityEventRead(ctx, actor, "e1"), store.ErrRecordNotFound,
			"your own action is not in your feed, so there is nothing there to mark read")
		require.ErrorIs(t, s.MarkActivityEventRead(ctx, actor, "nope"), store.ErrRecordNotFound,
			"an unknown id is the same answer, so it cannot be used to probe for events")
	})

	t.Run("read state is private to the reader who marked it", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		other := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "shared", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, other))
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
			{Id: "e2", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "rating_added"},
		}))

		require.NoError(t, s.MarkActivityEventRead(ctx, reader, "e1"))

		require.Equal(t, map[string]bool{"e1": true, "e2": false}, readStateById(t, s, reader))
		require.Equal(t, map[string]bool{"e1": false, "e2": false}, readStateById(t, s, other),
			"one member reading an event must not read it for another")

		n, err := s.GetActivityUnreadCount(ctx, other)
		require.NoError(t, err)
		require.EqualValues(t, 2, n, "the other member's badge is untouched")
	})

	t.Run("marking all read clears the badge and a later event raises it again", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "markall", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))
		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e1", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
			{Id: "e2", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "rating_added"},
		}))

		require.NoError(t, s.MarkActivityEventRead(ctx, reader, "e1"))
		require.NoError(t, s.MarkAllActivityEventsRead(ctx, reader),
			"mark-all must tolerate events that are already read")

		n, err := s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.Zero(t, n, "marking everything read clears the badge")
		require.Equal(t, map[string]bool{"e1": true, "e2": true}, readStateById(t, s, reader))

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{Id: "e3", GroupId: group.Id, ActorId: actor, ActorName: "a", Kind: "title_added"},
		}))
		n, err = s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.EqualValues(t, 1, n, "an event recorded after the sweep is unread")

		require.NoError(t, s.MarkAllActivityEventsRead(ctx, actor),
			"a sweep by someone with nothing to read is a no-op, not an error")
		n, err = s.GetActivityUnreadCount(ctx, reader)
		require.NoError(t, err)
		require.EqualValues(t, 1, n, "one user's sweep must not read anything for another")
	})

	t.Run("the cursor pages without duplicates or gaps", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		actor := addTestUser(t, s)
		reader := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "paging", actor))
		require.NoError(t, err)
		require.NoError(t, s.AddUserToGroup(ctx, group.Id, actor, reader))

		events := make([]models.ActivityEvent, 0, 25)
		for i := 0; i < 25; i++ {
			events = append(events, models.ActivityEvent{
				Id: fmt.Sprintf("e%02d", i), GroupId: group.Id,
				ActorId: actor, ActorName: "a", Kind: "title_added",
			})
		}
		require.NoError(t, s.InsertActivityEvents(ctx, events))

		seen := map[string]int{}
		var before *int64
		for {
			page, err := s.GetActivityFeed(ctx, reader, before, 10)
			require.NoError(t, err)
			if len(page) == 0 {
				break
			}
			for _, e := range page {
				seen[e.Id]++
			}
			last := page[len(page)-1].Seq
			before = &last
		}
		require.Len(t, seen, 25, "walking the cursor must return every event exactly once")
		for id, n := range seen {
			require.Equal(t, 1, n, "event %s was returned %d times", id, n)
		}
	})
}

func TestStore_InsertActivityEvents_Notifies(t *testing.T) {
	t.Run("a successful insert notifies with the event id", func(t *testing.T) {
		resetDB(t)
		ctx := context.Background()
		s := newTestStore(t)

		// A dedicated connection: LISTEN is per-connection, and a pooled one may
		// hand the LISTEN and the later wait to different sessions.
		conn, err := newTestPool(t).Acquire(ctx)
		require.NoError(t, err)
		defer conn.Release()
		_, err = conn.Exec(ctx, "LISTEN activity_events")
		require.NoError(t, err)

		actor := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "notify", actor))
		require.NoError(t, err, "failed to seed the group")

		require.NoError(t, s.InsertActivityEvents(ctx, []models.ActivityEvent{{
			GroupId: group.Id, ActorId: actor, ActorName: "Maria", Kind: "rating_added",
		}}))

		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		n, err := conn.Conn().WaitForNotification(waitCtx)
		require.NoError(t, err, "the insert must notify; a timeout here means it did not")
		require.Equal(t, "activity_events", n.Channel)
		require.NotEmpty(t, n.Payload, "the payload carries the event id")

		// The payload must name a row that exists — the listener will read it.
		got, err := s.GetActivityEventById(ctx, n.Payload)
		require.NoError(t, err)
		require.Equal(t, "rating_added", got.Kind)
	})

	t.Run("a rolled-back insert notifies nobody", func(t *testing.T) {
		// activity_events.group_id has a FK to groups(id), so a batch whose
		// second event carries a bogus group id fails after the first event
		// has already been inserted (and notified) inside the same
		// transaction — pinning that pg_notify's transactional delivery
		// really does suppress the notification on rollback, not just that
		// the doc comment says so.
		resetDB(t)
		ctx := context.Background()
		s := newTestStore(t)

		conn, err := newTestPool(t).Acquire(ctx)
		require.NoError(t, err)
		defer conn.Release()
		_, err = conn.Exec(ctx, "LISTEN activity_events")
		require.NoError(t, err)

		actor := addTestUser(t, s)
		group, err := s.CreateGroup(ctx, newTestGroup(t, "rollback", actor))
		require.NoError(t, err, "failed to seed the group")

		err = s.InsertActivityEvents(ctx, []models.ActivityEvent{
			{GroupId: group.Id, ActorId: actor, ActorName: "Maria", Kind: "rating_added"},
			{GroupId: "no-such-group", ActorId: actor, ActorName: "Maria", Kind: "rating_added"},
		})
		require.Error(t, err, "the second event's bogus group id must fail the whole batch")

		waitCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_, err = conn.Conn().WaitForNotification(waitCtx)
		require.Error(t, err, "a rolled-back insert must notify nobody; receiving a notification here means the first event's NOTIFY survived the rollback")

		var count int64
		require.NoError(t, newTestPool(t).QueryRow(ctx, "SELECT count(*) FROM activity_events").Scan(&count))
		require.Zero(t, count, "the rolled-back batch must leave no rows behind either")
	})
}

func TestStore_GetActivityEventById(t *testing.T) {
	t.Run("returns ErrRecordNotFound for a missing id", func(t *testing.T) {
		resetDB(t)
		ctx := context.Background()
		s := newTestStore(t)

		_, err := s.GetActivityEventById(ctx, "no-such-id")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}
