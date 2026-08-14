package activity

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestHub(t *testing.T) {
	t.Run("subscriber in the group receives", func(t *testing.T) {
		h := NewHub()
		sub := h.Subscribe("alice", []string{"g1"})
		defer h.Unsubscribe(sub)

		event := models.ActivityEvent{Id: "e1", GroupId: "g1", ActorId: "bob"}
		h.Publish(event)

		select {
		case got := <-sub.Events:
			require.Equal(t, event, got, "subscriber in the event's group should receive it unchanged")
		case <-time.After(time.Second):
			t.Fatal("expected an event, got none")
		}
	})

	t.Run("subscriber in a different group does not receive", func(t *testing.T) {
		h := NewHub()
		sub := h.Subscribe("alice", []string{"g2"})
		defer h.Unsubscribe(sub)

		h.Publish(models.ActivityEvent{Id: "e1", GroupId: "g1", ActorId: "bob"})

		select {
		case got := <-sub.Events:
			t.Fatalf("expected no event for a group the subscriber is not in, got %+v", got)
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("the actor does not receive their own event", func(t *testing.T) {
		h := NewHub()
		sub := h.Subscribe("alice", []string{"g1"})
		defer h.Unsubscribe(sub)

		h.Publish(models.ActivityEvent{Id: "e1", GroupId: "g1", ActorId: "alice"})

		select {
		case got := <-sub.Events:
			t.Fatalf("actor should not be notified about their own action, got %+v", got)
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("a full channel drops rather than blocking", func(t *testing.T) {
		h := NewHub()
		sub := h.Subscribe("alice", []string{"g1"})
		defer h.Unsubscribe(sub)

		for i := range subscriberBufferSize {
			h.Publish(models.ActivityEvent{Id: fmt.Sprintf("e%d", i), GroupId: "g1", ActorId: "bob"})
		}
		require.Len(t, sub.Events, subscriberBufferSize, "buffer should be saturated before the overflow publish")

		done := make(chan struct{})
		go func() {
			h.Publish(models.ActivityEvent{Id: "overflow", GroupId: "g1", ActorId: "bob"})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Publish blocked on a full subscriber channel instead of dropping")
		}

		require.Len(t, sub.Events, subscriberBufferSize, "the overflow event should have been dropped, not queued")
	})

	t.Run("Unsubscribe closes the channel and is safe to call twice", func(t *testing.T) {
		h := NewHub()
		sub := h.Subscribe("alice", []string{"g1"})

		h.Unsubscribe(sub)

		_, ok := <-sub.Events
		require.False(t, ok, "Events should be closed after Unsubscribe")

		require.NotPanics(t, func() { h.Unsubscribe(sub) }, "a second Unsubscribe must not panic")
	})

	t.Run("concurrent Publish, Subscribe and Unsubscribe", func(t *testing.T) {
		h := NewHub()
		stop := make(chan struct{})
		var wg sync.WaitGroup

		for i := range 4 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for j := 0; ; j++ {
					select {
					case <-stop:
						return
					default:
						h.Publish(models.ActivityEvent{
							Id:      fmt.Sprintf("p%d-%d", i, j),
							GroupId: "g1",
							ActorId: "bob",
						})
					}
				}
			}(i)
		}

		for i := range 4 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						sub := h.Subscribe(fmt.Sprintf("user%d", i), []string{"g1"})
						select {
						case <-sub.Events:
						default:
						}
						h.Unsubscribe(sub)
					}
				}
			}(i)
		}

		time.Sleep(200 * time.Millisecond)
		close(stop)
		wg.Wait()
	})
}
