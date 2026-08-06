package pgmigration

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/postgres"
)

// Result is the outcome of a verification pass. Failures are hard mismatches
// (wrong counts, missing rows, field differences); Warnings are known
// Mongo-side inconsistencies that cannot round-trip (membership drift).
type Result struct {
	Failures []string
	Warnings []string
	Checked  int
}

func (r *Result) OK() bool { return len(r.Failures) == 0 }

func (r *Result) failf(format string, args ...any) {
	r.Failures = append(r.Failures, fmt.Sprintf(format, args...))
}

func (r *Result) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Verify re-reads everything the migration wrote and compares it against the
// dump: table counts first, then every record through the real
// postgres.Store getters (deleted groups through row-level reads, since the
// store filters them out). Read errors are recorded as failures.
func Verify(ctx context.Context, pool *pgxpool.Pool, dump *Dump) *Result {
	res := &Result{}
	st := postgres.New(pool)
	q := database.New(pool)

	verifyCounts(ctx, res, pool, dump)
	verifyUsers(ctx, res, st, dump)
	verifyTitles(ctx, res, st, dump)
	verifyRatings(ctx, res, st, dump)
	verifyComments(ctx, res, st, dump)
	verifyGroups(ctx, res, st, q, dump)

	return res
}

// verifyCounts checks every table's row count against the dump.
func verifyCounts(ctx context.Context, res *Result, pool *pgxpool.Pool, dump *Dump) {
	ratingSeasons, commentSeasons := 0, 0
	for _, r := range dump.Ratings {
		if r.SeasonsRatings != nil {
			ratingSeasons += len(*r.SeasonsRatings)
		}
	}
	for _, c := range dump.Comments {
		if c.SeasonsComments != nil {
			commentSeasons += len(*c.SeasonsComments)
		}
	}
	members, groupTitles, groupTitleSeasons := 0, 0, 0
	for _, g := range dump.Groups {
		members += len(g.Users)
		groupTitles += len(g.Titles)
		for _, item := range g.Titles {
			if item.SeasonsWatched != nil {
				groupTitleSeasons += len(*item.SeasonsWatched)
			}
		}
	}

	expected := map[string]int{
		"users":               len(dump.Users),
		"titles":              len(dump.Titles),
		"ratings":             len(dump.Ratings),
		"rating_seasons":      ratingSeasons,
		"comments":            len(dump.Comments),
		"comment_seasons":     commentSeasons,
		"groups":              len(dump.Groups),
		"group_members":       members,
		"group_titles":        groupTitles,
		"group_title_seasons": groupTitleSeasons,
	}

	for _, table := range allTables {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			res.failf("count %s: %v", table, err)
			continue
		}
		if n != expected[table] {
			res.failf("%s: expected %d rows, found %d", table, expected[table], n)
		}
	}
}

func verifyUsers(ctx context.Context, res *Result, st *postgres.Store, dump *Dump) {
	for _, u := range dump.Users {
		got, err := st.GetUserById(ctx, u.Id)
		if err != nil {
			res.failf("user %s: read back failed: %v", u.Id, err)
			continue
		}
		res.Checked++

		// Postgres derives Groups from group_members (built from
		// group.users[], filtered to non-deleted groups). The hard check is
		// against that derivation; mongo's own user.groups[] copy drifting
		// from it is a warning (decision 6 in the spec).
		derived := derivedUserGroups(u.Id, dump.Groups)
		if !sameStringSet(got.Groups, derived) {
			res.failf("user %s: derived groups %v, postgres returned %v", u.Id, derived, got.Groups)
		}
		if !sameStringSet(u.Groups, derived) {
			res.warnf("user %s: mongo user.groups %v drifts from group membership %v", u.Id, u.Groups, derived)
		}

		want := mongodb.UserDbToModel(u)
		want.Groups, got.Groups = nil, nil
		if diff := diffModels(normalizeUser(want), normalizeUser(got)); diff != "" {
			res.failf("user %s: %s", u.Id, diff)
		}
	}
}

func verifyTitles(ctx context.Context, res *Result, st *postgres.Store, dump *Dump) {
	for _, t := range dump.Titles {
		got, err := st.GetTitleById(ctx, t.ID)
		if err != nil {
			res.failf("title %s: read back failed: %v", t.ID, err)
			continue
		}
		res.Checked++

		// TitleDbToModel already returns non-nil slices for the fields the
		// store's reader normalizes, so a plain deep-compare works.
		want := mongodb.TitleDbToModel(t)
		if diff := diffModels(normalizeTitle(want), normalizeTitle(got)); diff != "" {
			res.failf("title %s: %s", t.ID, diff)
		}
	}
}

func verifyRatings(ctx context.Context, res *Result, st *postgres.Store, dump *Dump) {
	for _, r := range dump.Ratings {
		got, err := st.GetRatingById(ctx, r.Id, r.UserId)
		if err != nil {
			res.failf("rating %s: read back failed: %v", r.Id, err)
			continue
		}
		res.Checked++

		want := mongodb.UserRatingDbToModel(r)
		if diff := diffModels(normalizeRating(want), normalizeRating(got)); diff != "" {
			res.failf("rating %s: %s", r.Id, diff)
		}
	}
}

func verifyComments(ctx context.Context, res *Result, st *postgres.Store, dump *Dump) {
	for _, c := range dump.Comments {
		got, err := st.GetCommentById(ctx, c.Id, c.UserId)
		if err != nil {
			res.failf("comment %s: read back failed: %v", c.Id, err)
			continue
		}
		res.Checked++

		want := mongodb.CommentDbToModel(c)
		if diff := diffModels(normalizeComment(want), normalizeComment(got)); diff != "" {
			res.failf("comment %s: %s", c.Id, diff)
		}
	}
}

func verifyGroups(ctx context.Context, res *Result, st *postgres.Store, q *database.Queries, dump *Dump) {
	for _, g := range dump.Groups {
		if !g.Deleted && len(g.Users) > 0 {
			got, err := st.GetGroupById(ctx, g.Id, g.Users[0])
			if err != nil {
				res.failf("group %s: read back failed: %v", g.Id, err)
				continue
			}
			res.Checked++

			want := mongodb.GroupDbToModel(g)
			if diff := diffModels(normalizeGroup(want), normalizeGroup(got)); diff != "" {
				res.failf("group %s: %s", g.Id, diff)
			}
			continue
		}

		// Deleted (or memberless) groups are invisible to the store's
		// readers — verify at row level instead.
		verifyGroupRows(ctx, res, q, g)
	}
}

// verifyGroupRows checks a group the store cannot read (deleted or
// memberless) directly against its rows.
func verifyGroupRows(ctx context.Context, res *Result, q *database.Queries, g mongodb.GroupDb) {
	row, err := q.GetGroupRowAnyById(ctx, g.Id)
	if err != nil {
		res.failf("group %s: row read failed: %v", g.Id, err)
		return
	}
	res.Checked++

	if row.Name != g.Name || row.Description != g.Description || row.OwnerID != g.OwnerId ||
		row.Deleted != g.Deleted ||
		!timePtrEqual(timestamptzPtr(row.DeletedAt), g.DeletedAt) ||
		!row.CreatedAt.Time.Equal(g.CreatedAt) || !row.UpdatedAt.Time.Equal(g.UpdatedAt) {
		res.failf("group %s: row mismatch: got %+v", g.Id, row)
	}

	memberIds, err := q.GetGroupMemberIds(ctx, g.Id)
	if err != nil {
		res.failf("group %s: member read failed: %v", g.Id, err)
	} else if !sameStringSet(memberIds, []string(g.Users)) {
		res.failf("group %s: members %v, expected %v", g.Id, memberIds, g.Users)
	}

	titleRows, err := q.GetGroupTitleRows(ctx, g.Id)
	if err != nil {
		res.failf("group %s: title rows read failed: %v", g.Id, err)
		return
	}
	if len(titleRows) != len(g.Titles) {
		res.failf("group %s: %d title rows, expected %d", g.Id, len(titleRows), len(g.Titles))
		return
	}
	for _, tr := range titleRows {
		item, ok := g.Titles[mongodb.TitleId(tr.TitleID)]
		if !ok {
			res.failf("group %s: unexpected title row %s", g.Id, tr.TitleID)
			continue
		}
		if tr.TitleType != item.TitleType || tr.Watched != item.Watched ||
			!timePtrEqual(timestamptzPtr(tr.WatchedAt), item.WatchedAt) ||
			!tr.AddedAt.Time.Equal(item.AddedAt) || !tr.UpdatedAt.Time.Equal(item.UpdatedAt) {
			res.failf("group %s title %s: row mismatch: got %+v", g.Id, tr.TitleID, tr)
		}
	}
}

// ----- normalization + comparison helpers -----

// diffModels deep-compares two already-normalized models and returns a
// human-readable diff, or "" when equal.
func diffModels(want, got any) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}
	return fmt.Sprintf("mismatch:\n    want: %+v\n    got:  %+v", want, got)
}

// utcPtr normalizes a *time.Time to UTC (BSON decodes to Local, pgx may use
// a different location; .UTC() makes DeepEqual safe).
func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func normalizeUser(u models.User) models.User {
	u.LastLoginAt = utcPtr(u.LastLoginAt)
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	if u.Groups != nil {
		g := append([]string(nil), u.Groups...)
		sort.Strings(g)
		u.Groups = g
	}
	return u
}

func normalizeTitle(t models.Title) models.Title {
	t.AddedAt = utcPtr(t.AddedAt)
	t.UpdatedAt = utcPtr(t.UpdatedAt)
	return t
}

func normalizeRating(r models.UserRating) models.UserRating {
	r.CreatedAt = r.CreatedAt.UTC()
	r.UpdatedAt = r.UpdatedAt.UTC()
	// A present-but-empty season map cannot round-trip (no child rows), and
	// the store reads zero rows back as nil — treat both as nil.
	if r.SeasonsRatings != nil && len(*r.SeasonsRatings) == 0 {
		r.SeasonsRatings = nil
	}
	if r.SeasonsRatings != nil {
		m := make(models.SeasonsRatings, len(*r.SeasonsRatings))
		for k, v := range *r.SeasonsRatings {
			v.AddedAt = v.AddedAt.UTC()
			v.UpdatedAt = v.UpdatedAt.UTC()
			m[k] = v
		}
		r.SeasonsRatings = &m
	}
	return r
}

func normalizeComment(c models.Comment) models.Comment {
	c.CreatedAt = c.CreatedAt.UTC()
	c.UpdatedAt = c.UpdatedAt.UTC()
	if c.SeasonsComments != nil && len(*c.SeasonsComments) == 0 {
		c.SeasonsComments = nil
	}
	if c.SeasonsComments != nil {
		m := make(models.SeasonsComments, len(*c.SeasonsComments))
		for k, v := range *c.SeasonsComments {
			v.AddedAt = v.AddedAt.UTC()
			v.UpdatedAt = v.UpdatedAt.UTC()
			m[k] = v
		}
		c.SeasonsComments = &m
	}
	return c
}

func normalizeGroup(g models.Group) models.Group {
	g.CreatedAt = g.CreatedAt.UTC()
	g.UpdatedAt = g.UpdatedAt.UTC()
	g.DeletedAt = utcPtr(g.DeletedAt)
	if g.Users != nil {
		u := append([]string(nil), g.Users...)
		sort.Strings(u)
		g.Users = u
	}
	// The store's reader always returns a non-nil titles map.
	if g.Titles == nil {
		g.Titles = models.GroupTitles{}
	}
	titles := make(models.GroupTitles, len(g.Titles))
	for id, item := range g.Titles {
		item.AddedAt = item.AddedAt.UTC()
		item.UpdatedAt = item.UpdatedAt.UTC()
		item.WatchedAt = utcPtr(item.WatchedAt)
		if item.SeasonsWatched != nil && len(*item.SeasonsWatched) == 0 {
			item.SeasonsWatched = nil
		}
		if item.SeasonsWatched != nil {
			m := make(models.SeasonsWatched, len(*item.SeasonsWatched))
			for k, v := range *item.SeasonsWatched {
				v.AddedAt = v.AddedAt.UTC()
				v.UpdatedAt = v.UpdatedAt.UTC()
				v.WatchedAt = utcPtr(v.WatchedAt)
				m[k] = v
			}
			item.SeasonsWatched = &m
		}
		titles[id] = item
	}
	g.Titles = titles
	return g
}

// derivedUserGroups returns the non-deleted groups that list userId as a
// member — exactly what postgres derives from group_members.
func derivedUserGroups(userId string, groups []mongodb.GroupDb) []string {
	var out []string
	for _, g := range groups {
		if g.Deleted {
			continue
		}
		for _, u := range g.Users {
			if u == userId {
				out = append(out, g.Id)
				break
			}
		}
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func timePtrEqual(a, b *time.Time) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || a.Equal(*b)
}

// timestamptzPtr converts a nullable pgtype.Timestamptz to *time.Time.
func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
