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

// TestNoteValueRoundsAwayFloat32Noise pins the reason noteValue exists.
//
// A note is a one-decimal value by contract, but it is stored in a REAL
// column, so float64(float32(5.6)) is 5.599999904632568 — and encoding/json
// writes all of it. A feed line read "rated 1917 with note 5.599999904632568"
// before this. Each case below is a value whose float32 representation is not
// exact; the assertion is that the payload carries the note the user typed.
func TestNoteValueRoundsAwayFloat32Noise(t *testing.T) {
	for _, tc := range []struct {
		name string
		note float32
		want float64
	}{
		{"the reported case", 5.6, 5.6},
		{"another inexact tenth", 8.7, 8.7},
		{"a half is exact in binary and must survive", 5.5, 5.5},
		{"zero", 0, 0},
		{"the maximum", 10, 10},
		{"one decimal at the bottom of the range", 0.1, 0.1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := noteValue(tc.note); got != tc.want {
				t.Errorf("noteValue(%v) = %v, want %v", tc.note, got, tc.want)
			}
		})
	}
}

// TestRatingEventsCarryRoundedNotes checks the constructors actually use it,
// rather than the helper existing while a call site still widens directly.
func TestRatingEventsCarryRoundedNotes(t *testing.T) {
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
