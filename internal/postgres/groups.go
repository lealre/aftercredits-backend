package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// assembleGroupTitles fetches every group_title row for groupId plus all of
// their group_title_seasons in one batched query and assembles the
// models.GroupTitles map. The map is always non-nil (possibly empty) — a group
// always reports a titles map, even when it holds none; each item's
// SeasonsWatched is nil when it has no season rows.
func (s *Store) assembleGroupTitles(ctx context.Context, groupId string) (models.GroupTitles, error) {
	titleRows, err := s.q.GetGroupTitleRows(ctx, groupId)
	if err != nil {
		return nil, err
	}

	seasonRows, err := s.q.GetGroupTitleSeasonRows(ctx, groupId)
	if err != nil {
		return nil, err
	}

	seasonsByTitle := make(map[string][]database.GroupTitleSeason, len(titleRows))
	for _, sr := range seasonRows {
		seasonsByTitle[sr.TitleID] = append(seasonsByTitle[sr.TitleID], sr)
	}

	titles := make(models.GroupTitles, len(titleRows))
	for _, tr := range titleRows {
		titles[tr.TitleID] = groupTitleRowToModel(tr, assembleSeasonsWatched(seasonsByTitle[tr.TitleID]))
	}
	return titles, nil
}

// CreateGroup inserts a new group row plus a group_members row for every user
// in group.Users (the owner) in a single transaction. The id and timestamps are
// generated here, not taken from the caller-supplied group. A violation of the (owner_id, name) partial
// unique index is reported as store.ErrDuplicatedRecord.
func (s *Store) CreateGroup(ctx context.Context, group models.Group) (models.Group, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Group{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	id := uuid.NewString()
	now := time.Now()

	row, err := qtx.InsertGroup(ctx, database.InsertGroupParams{
		ID:          id,
		Name:        group.Name,
		Description: group.Description,
		OwnerID:     group.OwnerId,
		CreatedAt:   timeToTimestamptz(now),
		UpdatedAt:   timeToTimestamptz(now),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return models.Group{}, store.ErrDuplicatedRecord
		}
		return models.Group{}, err
	}

	for _, userId := range group.Users {
		if err := qtx.AddGroupMember(ctx, database.AddGroupMemberParams{
			GroupID: id,
			UserID:  userId,
		}); err != nil {
			return models.Group{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Group{}, err
	}

	// Echo the input Users/Titles rather than re-reading them: Titles stays the
	// empty-but-non-nil map the caller supplied, Users stays the owner-only slice.
	return groupRowToModel(row, group.Users, group.Titles), nil
}

// GroupExists reports whether a non-deleted group with the given id exists and
// has userId as a member.
func (s *Store) GroupExists(ctx context.Context, groupId, userId string) (bool, error) {
	return s.q.GroupExists(ctx, database.GroupExistsParams{ID: groupId, UserID: userId})
}

// GroupContainsTitle reports whether a non-deleted group with the given id has
// userId as a member and contains titleId.
func (s *Store) GroupContainsTitle(ctx context.Context, groupId, titleId, userId string) (bool, error) {
	return s.q.GroupContainsTitle(ctx, database.GroupContainsTitleParams{
		ID:      groupId,
		TitleID: titleId,
		UserID:  userId,
	})
}

// GetGroupById fetches a non-deleted group that userId is a member of and
// assembles its members and titles (with per-title seasons). A missing/deleted
// group, or one userId is not a member of, is reported as
// store.ErrRecordNotFound.
func (s *Store) GetGroupById(ctx context.Context, groupId, userId string) (models.Group, error) {
	row, err := s.q.GetGroupRow(ctx, database.GetGroupRowParams{ID: groupId, UserID: userId})
	if err != nil {
		return models.Group{}, notFound(err)
	}

	users, err := s.q.GetGroupMemberIds(ctx, groupId)
	if err != nil {
		return models.Group{}, err
	}

	titles, err := s.assembleGroupTitles(ctx, groupId)
	if err != nil {
		return models.Group{}, err
	}

	return groupRowToModel(row, users, titles), nil
}

// AddUserToGroup adds userToAddId as a member of the group, but only if
// ownerId is already a member of it. The insert is idempotent
// (ON CONFLICT DO NOTHING), so re-adding an existing member is a no-op. A group
// ownerId is not a member of is reported as store.ErrRecordNotFound.
func (s *Store) AddUserToGroup(ctx context.Context, groupId, ownerId, userToAddId string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	ok, err := qtx.GroupHasMember(ctx, database.GroupHasMemberParams{GroupID: groupId, UserID: ownerId})
	if err != nil {
		return err
	}
	if !ok {
		return store.ErrRecordNotFound
	}

	if err := qtx.AddGroupMember(ctx, database.AddGroupMemberParams{
		GroupID: groupId,
		UserID:  userToAddId,
	}); err != nil {
		return err
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetUsersFromGroup returns the full member users of a non-deleted group that
// userId is a member of (which fetches the
// group then loads each member user with its own groups). A missing/deleted
// group, or one userId is not a member of, is reported as
// store.ErrRecordNotFound.
func (s *Store) GetUsersFromGroup(ctx context.Context, groupId, userId string) ([]models.User, error) {
	if _, err := s.q.GetGroupRow(ctx, database.GetGroupRowParams{ID: groupId, UserID: userId}); err != nil {
		return []models.User{}, notFound(err)
	}

	rows, err := s.q.GetGroupMemberUsers(ctx, groupId)
	if err != nil {
		return []models.User{}, err
	}

	users := make([]models.User, 0, len(rows))
	for _, row := range rows {
		user, err := s.loadUser(ctx, row)
		if err != nil {
			return []models.User{}, err
		}
		users = append(users, user)
	}
	return users, nil
}

// AddNewGroupTitle adds titleId to the group as a movie with watched=false.
// Adding a title that is already present overwrites the existing entry and
// resets its addedAt.
func (s *Store) AddNewGroupTitle(ctx context.Context, groupId string, titleId string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	now := timeToTimestamptz(time.Now())
	if _, err := qtx.UpsertGroupTitle(ctx, database.UpsertGroupTitleParams{
		GroupID:   groupId,
		TitleID:   titleId,
		TitleType: "movie",
		Watched:   false,
		WatchedAt: ptrToTimestamptz(nil),
		AddedAt:   now,
		UpdatedAt: now,
	}); err != nil {
		return err
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdateGroupTitleWatchedForMovie sets the top-level watched/watchedAt on a
// group's title: watched and
// watchedAt are each only touched when their argument is non-nil, and a
// watchedAt argument whose Time is nil clears the column to NULL. Returns the
// updated item. A missing title is reported as store.ErrRecordNotFound, and no
// updatable fields at all as an error.
func (s *Store) UpdateGroupTitleWatchedForMovie(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (*models.GroupTitleItem, error) {
	if watched == nil && watchedAt == nil {
		return nil, fmt.Errorf("no fields to update")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	current, err := qtx.GetGroupTitleRow(ctx, database.GetGroupTitleRowParams{GroupID: groupId, TitleID: titleId})
	if err != nil {
		return nil, notFound(err)
	}

	newWatched := current.Watched
	if watched != nil {
		newWatched = *watched
	}

	newWatchedAt := current.WatchedAt
	if watchedAt != nil {
		newWatchedAt = ptrToTimestamptz(watchedAt.Time)
	}

	row, err := qtx.UpdateGroupTitleWatchedRow(ctx, database.UpdateGroupTitleWatchedRowParams{
		GroupID:   groupId,
		TitleID:   titleId,
		Watched:   newWatched,
		WatchedAt: newWatchedAt,
		UpdatedAt: timeToTimestamptz(time.Now()),
	})
	if err != nil {
		return nil, notFound(err)
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return nil, err
	}

	seasonRows, err := qtx.GetGroupTitleSeasonRowsForTitle(ctx, database.GetGroupTitleSeasonRowsForTitleParams{
		GroupID: groupId,
		TitleID: titleId,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	item := groupTitleRowToModel(row, assembleSeasonsWatched(seasonRows))
	return &item, nil
}

// UpdateGroupTitleWatchedForTVSeries upserts a single season's watched/watchedAt
// for a group's title, then recomputes the title's top-level watched/watchedAt
// from all of its seasons, all in one transaction.
//
// Recompute rule: top-level watched is true
// if AT LEAST ONE season is watched; top-level watchedAt is the LATEST (max)
// watchedAt among the watched seasons, or NULL when no watched season carries a
// date. addedAt is only stamped on a season the first time it is seen.
//
// The group must exist, be non-deleted, and have userId as a member, and the
// title must be present; otherwise store.ErrRecordNotFound is returned.
func (s *Store) UpdateGroupTitleWatchedForTVSeries(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate, season int, userId string) (*models.GroupTitleItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	// Membership + not-deleted guard (the store's GetGroupById), then confirm
	// the title is present in the group.
	if _, err := qtx.GetGroupRow(ctx, database.GetGroupRowParams{ID: groupId, UserID: userId}); err != nil {
		return nil, notFound(err)
	}
	if _, err := qtx.GetGroupTitleRow(ctx, database.GetGroupTitleRowParams{GroupID: groupId, TitleID: titleId}); err != nil {
		return nil, notFound(err)
	}

	now := time.Now()
	seasonKey := fmt.Sprintf("%d", season)

	// Load the existing season row (if any) to preserve addedAt and the fields
	// the caller did not supply.
	existing, err := qtx.GetGroupTitleSeasonRow(ctx, database.GetGroupTitleSeasonRowParams{
		GroupID: groupId,
		TitleID: titleId,
		Season:  seasonKey,
	})
	seasonExists := err == nil
	if err != nil && notFound(err) != store.ErrRecordNotFound {
		return nil, err
	}

	seasonWatched := false
	seasonWatchedAt := ptrToTimestamptz(nil)
	addedAt := timeToTimestamptz(now)
	if seasonExists {
		seasonWatched = existing.Watched
		seasonWatchedAt = existing.WatchedAt
		addedAt = existing.AddedAt
	}
	if watched != nil {
		seasonWatched = *watched
	}
	if watchedAt != nil {
		seasonWatchedAt = ptrToTimestamptz(watchedAt.Time)
	}

	if _, err := qtx.UpsertGroupTitleSeason(ctx, database.UpsertGroupTitleSeasonParams{
		GroupID:   groupId,
		TitleID:   titleId,
		Season:    seasonKey,
		Watched:   seasonWatched,
		WatchedAt: seasonWatchedAt,
		AddedAt:   addedAt,
		UpdatedAt: timeToTimestamptz(now),
	}); err != nil {
		return nil, err
	}

	// Recompute the top-level watched/watchedAt from every season.
	seasonRows, err := qtx.GetGroupTitleSeasonRowsForTitle(ctx, database.GetGroupTitleSeasonRowsForTitleParams{
		GroupID: groupId,
		TitleID: titleId,
	})
	if err != nil {
		return nil, err
	}

	topWatched := false
	var topWatchedAt *time.Time
	for _, sr := range seasonRows {
		if sr.Watched {
			topWatched = true
			if sr.WatchedAt.Valid {
				w := sr.WatchedAt.Time
				if topWatchedAt == nil || w.After(*topWatchedAt) {
					topWatchedAt = &w
				}
			}
		}
	}

	row, err := qtx.UpdateGroupTitleWatchedRow(ctx, database.UpdateGroupTitleWatchedRowParams{
		GroupID:   groupId,
		TitleID:   titleId,
		Watched:   topWatched,
		WatchedAt: ptrToTimestamptz(topWatchedAt),
		UpdatedAt: timeToTimestamptz(now),
	})
	if err != nil {
		return nil, notFound(err)
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	item := groupTitleRowToModel(row, assembleSeasonsWatched(seasonRows))
	return &item, nil
}

// UpdateGroupInfo sets name and description on a non-deleted group.
// A violation of the (owner_id, name) partial unique
// index is reported as store.ErrDuplicatedRecord; a missing/deleted group as
// store.ErrRecordNotFound.
func (s *Store) UpdateGroupInfo(ctx context.Context, groupId, name, description string) error {
	n, err := s.q.UpdateGroupInfoRow(ctx, database.UpdateGroupInfoRowParams{
		ID:          groupId,
		Name:        name,
		Description: description,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	if n == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

// SoftDeleteGroup marks a non-deleted group as deleted.
// An already-deleted or missing group is reported as
// store.ErrRecordNotFound.
func (s *Store) SoftDeleteGroup(ctx context.Context, groupId string) error {
	n, err := s.q.SoftDeleteGroupRow(ctx, groupId)
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

// RemoveUserFromGroup removes userId from a non-deleted group's members,
// as follows: the not-found error keys off the group
// (missing/deleted), not off whether userId was actually a member.
func (s *Store) RemoveUserFromGroup(ctx context.Context, groupId, userId string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	ok, err := qtx.GroupExistsNotDeleted(ctx, groupId)
	if err != nil {
		return err
	}
	if !ok {
		return store.ErrRecordNotFound
	}

	if err := qtx.RemoveGroupMember(ctx, database.RemoveGroupMemberParams{
		GroupID: groupId,
		UserID:  userId,
	}); err != nil {
		return err
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// groupTitlesOrderKeys is the sort-key whitelist for GetGroupTitlesPage.
// Unknown keys normalize to "" (primary_title), keeping the requested
// direction — the same fallback GetTitlesPage applies.
var groupTitlesOrderKeys = map[string]bool{
	"": true, "primaryTitle": true, "imdbRating": true, "startYear": true,
	"type": true, "voteCount": true, "updatedAt": true,
	"watched": true, "watchedAt": true, "addedAt": true,
}

// GroupHasTitleEntries reports whether the group holds any title entry
// matching the same filters GetGroupTitlesPage applies, counting entries whose
// title is no longer in the catalogue (GetGroupTitlesPage's INNER JOIN hides
// those, so its total alone cannot tell an empty group from a fully orphaned
// one). See the query comment in sql/queries/groups.sql for why the join side
// is a LEFT JOIN and what the caller does with the answer.
func (s *Store) GroupHasTitleEntries(ctx context.Context, groupId string, watched *bool, titleTypes []string) (bool, error) {
	var watchedArg pgtype.Bool
	if watched != nil {
		watchedArg = pgtype.Bool{Bool: *watched, Valid: true}
	}
	return s.q.GroupHasTitleEntries(ctx, database.GroupHasTitleEntriesParams{
		GroupID:    groupId,
		Watched:    watchedArg,
		TitleTypes: titleTypes, // nil slice -> SQL NULL -> filter off
	})
}

// GetGroupTitlesPage returns one page of a group's titles — full title plus
// this group's watch-state, seasons stitched in — with the post-filter total.
// Filters are nil-defaulted; the ORDER BY is total (ends in t.id ASC).
func (s *Store) GetGroupTitlesPage(ctx context.Context, groupId string, watched *bool, titleTypes []string, orderBy string, ascending *bool, size, page int) ([]models.GroupPagedTitle, int64, error) {
	if !groupTitlesOrderKeys[orderBy] {
		orderBy = ""
	}
	descending := ascending != nil && !*ascending

	var watchedArg pgtype.Bool
	if watched != nil {
		watchedArg = pgtype.Bool{Bool: *watched, Valid: true}
	}

	// Whenever no row can be returned, the window-function total goes with
	// them, so the total has to come from the companion count over the same
	// WHERE. Every such exit uses this.
	emptyPage := func() ([]models.GroupPagedTitle, int64, error) {
		total, err := s.q.CountGroupTitles(ctx, database.CountGroupTitlesParams{
			GroupID: groupId, Watched: watchedArg, TitleTypes: titleTypes,
		})
		if err != nil {
			return nil, 0, err
		}
		return []models.GroupPagedTitle{}, total, nil
	}

	// A non-positive size or an offset that would overflow int64 selects no
	// row at all (see pageOffset): same fallback as a past-the-end page takes
	// below — an empty page with the correct total, never an error.
	offset, ok := pageOffset(size, page)
	if !ok {
		return emptyPage()
	}

	rows, err := s.q.GetGroupTitlesPage(ctx, database.GetGroupTitlesPageParams{
		GroupID:    groupId,
		Watched:    watchedArg,
		TitleTypes: titleTypes, // nil slice -> SQL NULL -> filter off
		OrderBy:    orderBy,
		Descending: descending,
		PageSize:   int64(size),
		PageOffset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		// Empty result set and past-the-end page look identical here; the
		// companion count keeps the total correct either way.
		return emptyPage()
	}
	total := rows[0].TotalCount

	// group_title_seasons rows are only ever written for a TV series — the
	// season watched-update refuses any other title type — so a page holding
	// no series cannot have any, and asking for them is a round trip whose
	// answer is known to be empty. The page rows already carry the type, so
	// the decision costs nothing.
	//
	// Leaving seasonsByTitle nil in that case is deliberate and safe:
	// indexing a nil map yields the nil slice, which assembleSeasonsWatched
	// maps to a nil SeasonsWatched — exactly the "no season rows" shape the
	// contract requires (CONVENTIONS §5: the season maps are nil when absent,
	// never an empty map), and the same value the query would have produced.
	hasSeries := false
	for _, r := range rows {
		if models.IsSeriesTitleType(r.Type) {
			hasSeries = true
			break
		}
	}

	var seasonsByTitle map[string][]database.GroupTitleSeason
	if hasSeries {
		titleIds := make([]string, 0, len(rows))
		for _, r := range rows {
			titleIds = append(titleIds, r.ID)
		}
		seasonRows, err := s.q.GetGroupTitleSeasonRowsForTitles(ctx, database.GetGroupTitleSeasonRowsForTitlesParams{
			GroupID: groupId, TitleIds: titleIds,
		})
		if err != nil {
			return nil, 0, err
		}
		seasonsByTitle = make(map[string][]database.GroupTitleSeason, len(seasonRows))
		for _, sr := range seasonRows {
			seasonsByTitle[sr.TitleID] = append(seasonsByTitle[sr.TitleID], sr)
		}
	}

	out := make([]models.GroupPagedTitle, 0, len(rows))
	for _, r := range rows {
		title, err := rowToTitle(database.Title{
			ID: r.ID, PrimaryTitle: r.PrimaryTitle, Type: r.Type,
			StartYear: r.StartYear, RatingAggregate: r.RatingAggregate,
			VoteCount: r.VoteCount, AddedAt: r.AddedAt, UpdatedAt: r.UpdatedAt,
			Metadata: r.Metadata,
		})
		if err != nil {
			return nil, 0, err
		}
		out = append(out, models.GroupPagedTitle{
			Title: title,
			Item: models.GroupTitleItem{
				TitleId: r.ID,
				// Deliberately titles.type, NOT group_titles.title_type.
				// The two are independent stores of the same fact and they
				// disagree in practice: title_type is stamped once at
				// add-to-group time — always the literal "movie", see
				// AddNewGroupTitle — and never refreshed, while titles.type
				// tracks the catalogue. On the production database 98 of 122
				// group_titles rows are wrong under the old source (88 hold
				// the schema default "" and 10 say "movie" for a
				// tvSeries/tvMiniSeries), so reading it back would report a
				// series as a movie. This page is also filtered by
				// titles.type (see GetGroupTitlesPage in
				// sql/queries/groups.sql), and a row must not claim a type
				// its own filter contradicts.
				TitleType:      title.Type,
				SeasonsWatched: assembleSeasonsWatched(seasonsByTitle[r.ID]),
				Watched:        r.GtWatched,
				AddedAt:        r.GtAddedAt.Time,
				UpdatedAt:      r.GtUpdatedAt.Time,
				WatchedAt:      timestamptzToPtr(r.GtWatchedAt),
			},
		})
	}
	return out, total, nil
}

// RemoveTitleFromGroup removes titleId from a group userId is a member of (its
// seasons cascade): the not-found error
// keys off group membership, not off whether the title was actually present.
func (s *Store) RemoveTitleFromGroup(ctx context.Context, groupId, titleId, userId string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	ok, err := qtx.GroupHasMember(ctx, database.GroupHasMemberParams{GroupID: groupId, UserID: userId})
	if err != nil {
		return err
	}
	if !ok {
		return store.ErrRecordNotFound
	}

	if _, err := qtx.DeleteGroupTitle(ctx, database.DeleteGroupTitleParams{
		GroupID: groupId,
		TitleID: titleId,
	}); err != nil {
		return err
	}

	if err := qtx.TouchGroup(ctx, groupId); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
