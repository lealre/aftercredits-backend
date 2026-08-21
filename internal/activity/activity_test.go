package activity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecorder(t *testing.T) {
	t.Run("Record on a context without a recorder is a no-op", func(t *testing.T) {
		require.NotPanics(t, func() {
			Record(context.Background(), RatingAdded("g1", "tt1", "Dune", 9, nil))
		}, "a write path used outside an HTTP request must not panic")
		require.Empty(t, Recorded(context.Background()))
	})

	t.Run("recorded events come back in order", func(t *testing.T) {
		ctx := WithRecorder(context.Background())
		Record(ctx, TitleAdded("g1", "tt1", "Dune"))
		Record(ctx, RatingAdded("g1", "tt1", "Dune", 9, nil))

		got := Recorded(ctx)
		require.Len(t, got, 2)
		require.Equal(t, KindTitleAdded, got[0].Kind)
		require.Equal(t, KindRatingAdded, got[1].Kind)
		require.Equal(t, float64(9), got[1].Payload["note"])
	})
}

// TestRatingEventsCarryExactNotes pins that the four rating constructors put
// the note straight into the payload, unrounded, now that there is nothing
// left to round away. Before the note path became float64 end to end
// (backed by a NUMERIC(3,1) column), these same values arrived here already
// widened from a REAL-stored float32 — 5.6 became 5.599999904632568 — and a
// noteValue helper rounded it back on the way into the payload. That helper
// is gone; this test is its replacement, proving the constructors need no
// such repair because nothing upstream of them is imprecise anymore. Each
// value below is one that float32 could not have represented exactly, which
// is exactly the case the old helper existed to paper over.
func TestRatingEventsCarryExactNotes(t *testing.T) {
	season := 2

	added := RatingAdded("g", "tt1", "T", 5.6, nil)
	if added.Payload["note"] != 5.6 {
		t.Errorf("RatingAdded note = %v, want 5.6", added.Payload["note"])
	}

	updated := RatingUpdated("g", "tt1", "T", 5.6, 8.7, &season)
	if updated.Payload["note"] != 5.6 {
		t.Errorf("RatingUpdated note = %v, want 5.6", updated.Payload["note"])
	}
	if updated.Payload["previousNote"] != 8.7 {
		t.Errorf("RatingUpdated previousNote = %v, want 8.7", updated.Payload["previousNote"])
	}

	deleted := RatingDeleted("g", "tt1", "T", 5.6)
	if deleted.Payload["previousNote"] != 5.6 {
		t.Errorf("RatingDeleted previousNote = %v, want 5.6", deleted.Payload["previousNote"])
	}

	seasonDeleted := RatingSeasonDeleted("g", "tt1", "T", season, 8.7)
	if seasonDeleted.Payload["previousNote"] != 8.7 {
		t.Errorf("RatingSeasonDeleted previousNote = %v, want 8.7", seasonDeleted.Payload["previousNote"])
	}
}
