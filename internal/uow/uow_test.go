package uow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnitOfWork_Context(t *testing.T) {
	t.Run("FromContext returns nil when no unit of work was seeded", func(t *testing.T) {
		require.Nil(t, FromContext(context.Background()),
			"a request without a unit of work must report none, so the store falls back to the pool")
	})

	t.Run("Active is nil until a transaction is actually begun", func(t *testing.T) {
		u := New(nil)
		ctx := With(context.Background(), u)
		require.Same(t, u, FromContext(ctx), "the seeded unit of work must come back out")
		require.Nil(t, u.Active(),
			"no transaction may be begun before the first write — a handler that calls an external API before writing must not hold one")
	})
}
