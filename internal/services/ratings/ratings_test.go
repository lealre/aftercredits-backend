package ratings

import (
	"testing"

	"github.com/lealre/movies-backend/internal/models"
)

// TestOverallNote pins overallNote's rounding directly against the pure Go
// function, independent of the database.
//
// The integration-level equivalent (tests/ratings_test.go's "the derived TV
// series overall note is rounded to one decimal") can no longer tell this
// function's rounding apart from the database's: since 009 made the note
// column NUMERIC(3,1), Postgres itself rounds any excess scale on write, so
// an unrounded mean handed to the store comes back rounded regardless of
// whether overallNote did its job first. A mutation that deletes
// roundToOneDecimal's rounding entirely still passes that integration test —
// the column quietly repairs the mistake before it is ever persisted. This
// test calls overallNote directly, with no store and no database in the
// path, so a regression here has nowhere to hide.
func TestOverallNote(t *testing.T) {
	t.Run("nil seasons map yields zero", func(t *testing.T) {
		if got := overallNote(nil); got != 0 {
			t.Errorf("overallNote(nil) = %v, want 0", got)
		}
	})

	t.Run("empty seasons map yields zero", func(t *testing.T) {
		empty := models.SeasonsRatings{}
		if got := overallNote(&empty); got != 0 {
			t.Errorf("overallNote(empty) = %v, want 0", got)
		}
	})

	t.Run("a mean that is already one decimal is returned unchanged", func(t *testing.T) {
		seasons := models.SeasonsRatings{
			"1": {Rating: 5},
			"2": {Rating: 8},
		}
		if got := overallNote(&seasons); got != 6.5 {
			t.Errorf("overallNote({5,8}) = %v, want 6.5", got)
		}
	})

	t.Run("a mean that is not one decimal is rounded to one", func(t *testing.T) {
		// (5 + 8 + 10) / 3 = 7.666..., which must come back as 7.7.
		seasons := models.SeasonsRatings{
			"1": {Rating: 5},
			"2": {Rating: 8},
			"3": {Rating: 10},
		}
		if got := overallNote(&seasons); got != 7.7 {
			t.Errorf("overallNote({5,8,10}) = %v, want 7.7", got)
		}

		// (5 + 8 + 9) / 3 = 7.333..., which must come back as 7.3.
		seasons["3"] = models.SeasonRatingItem{Rating: 9}
		if got := overallNote(&seasons); got != 7.3 {
			t.Errorf("overallNote({5,8,9}) = %v, want 7.3", got)
		}
	})
}

// TestRoundToOneDecimal exercises the rounding helper on its own, including
// the halfway case overallNote's mean can actually produce.
func TestRoundToOneDecimal(t *testing.T) {
	for _, tc := range []struct {
		note float64
		want float64
	}{
		{7.666666666666667, 7.7},
		{7.333333333333333, 7.3},
		{6.5, 6.5},
		{0, 0},
		{10, 10},
	} {
		if got := roundToOneDecimal(tc.note); got != tc.want {
			t.Errorf("roundToOneDecimal(%v) = %v, want %v", tc.note, got, tc.want)
		}
	}
}
