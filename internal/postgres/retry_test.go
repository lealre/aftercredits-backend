package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The schedule is the contract: it has to be long enough to cover a Pi reboot
// where postgres is still starting, and short enough that a genuinely absent
// database is reported rather than hung on.
func TestBackoffSchedule(t *testing.T) {
	var got []time.Duration
	for i := range 8 {
		got = append(got, backoffFor(i))
	}
	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		10 * time.Second, // capped
		10 * time.Second,
		10 * time.Second,
		10 * time.Second,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("attempt %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRetryStopsOnFirstSuccess(t *testing.T) {
	calls := 0
	err := retryConnect(context.Background(), 5, func() error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	}, func(time.Duration) {})

	if err != nil {
		t.Fatalf("want success, got %v", err)
	}
	// Without this, a retry loop that ignores success and always runs to the
	// end of its budget would pass the failure test just as happily.
	if calls != 3 {
		t.Errorf("want 3 attempts (stop as soon as it works), got %d", calls)
	}
}

// The LAST error, not a generic one: the log line is the only thing an operator
// has, and "postgres unreachable" is far less useful than the driver saying the
// password was rejected.
func TestRetryReturnsTheLastError(t *testing.T) {
	boom := errors.New("password authentication failed")
	err := retryConnect(context.Background(), 3, func() error { return boom },
		func(time.Duration) {})

	if !errors.Is(err, boom) {
		t.Errorf("want the underlying error wrapped, got %v", err)
	}
}

func TestRetryGivesUpAfterTheBudget(t *testing.T) {
	calls := 0
	_ = retryConnect(context.Background(), 4, func() error {
		calls++
		return errors.New("nope")
	}, func(time.Duration) {})

	if calls != 4 {
		t.Errorf("want exactly 4 attempts, got %d", calls)
	}
}

// A cancelled context must abandon the wait immediately. Without this, a
// shutdown during startup would block for the whole remaining budget.
func TestRetryHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := retryConnect(ctx, 10, func() error {
		calls++
		cancel()
		return errors.New("nope")
	}, func(time.Duration) {})

	if err == nil {
		t.Fatal("want an error when the context is cancelled")
	}
	if calls != 1 {
		t.Errorf("want to stop after the cancelling attempt, got %d attempts", calls)
	}
}
