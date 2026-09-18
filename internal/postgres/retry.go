package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// connectAttempts x the schedule below is roughly a 60s window: long enough
	// to cover a reboot where postgres is still starting, short enough that a
	// genuinely absent database is reported rather than hung on forever.
	connectAttempts = 9

	backoffBase = 1 * time.Second
	backoffMax  = 10 * time.Second
)

// backoffFor returns the wait before the attempt after i, doubling to a cap.
func backoffFor(i int) time.Duration {
	d := backoffBase << i
	if d > backoffMax || d <= 0 { // <= 0 catches the shift overflowing
		return backoffMax
	}
	return d
}

// retryConnect runs attempt until it succeeds, the budget runs out, or ctx is
// cancelled. sleep is injected so the schedule can be tested without waiting.
//
// It returns the LAST error rather than a generic one: that line is the only
// thing an operator gets, and "authentication failed" is far more useful than
// "could not connect".
func retryConnect(ctx context.Context, attempts int, attempt func() error, sleep func(time.Duration)) error {
	var last error
	for i := range attempts {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cancelled while connecting to postgres: %w", err)
		}
		if last = attempt(); last == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cancelled while connecting to postgres: %w", err)
		}
		if i < attempts-1 {
			wait := backoffFor(i)
			slog.Warn("postgres not reachable, retrying",
				"err", last, "attempt", i+1, "of", attempts, "retry_in", wait.String())
			sleep(wait)
		}
	}
	return fmt.Errorf("postgres unreachable after %d attempts: %w", attempts, last)
}
