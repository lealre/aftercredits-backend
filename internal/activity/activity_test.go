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
