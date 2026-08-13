package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// activityRow is one row of the activity log, read straight from the database
// so emit-site tests do not depend on the feed API existing yet.
type activityRow struct {
	Kind      string
	GroupId   string
	ActorId   string
	ActorName string
	TitleName *string
}

// getActivityRows returns every recorded event, oldest first.
func getActivityRows(t *testing.T) []activityRow {
	t.Helper()
	rows, err := testPool.Query(context.Background(),
		`SELECT kind, group_id, actor_id, actor_name, title_name
		 FROM activity_events ORDER BY seq`)
	require.NoError(t, err, "failed to read the activity log")
	defer rows.Close()

	out := []activityRow{}
	for rows.Next() {
		var r activityRow
		require.NoError(t, rows.Scan(&r.Kind, &r.GroupId, &r.ActorId, &r.ActorName, &r.TitleName))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}
