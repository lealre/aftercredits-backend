package groups

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
	"github.com/lealre/movies-backend/internal/store"
)

func CreateGroup(db store.Store, ctx context.Context, req CreateGroupRequest, userId string) (GroupResponse, error) {

	if strings.TrimSpace(req.Name) == "" {
		return GroupResponse{}, ErrGroupNameInvalid
	}

	group := models.Group{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		OwnerId:     userId,
		Users:       []string{userId},
		Titles:      models.GroupTitles{},
	}

	newGroup, err := db.CreateGroup(ctx, group)
	if err != nil {
		if errors.Is(err, store.ErrDuplicatedRecord) {
			return GroupResponse{}, ErrGroupDuplicatedName
		}
		return GroupResponse{}, err
	}

	_, err = users.UpdateUserGroup(db, ctx, userId, newGroup.Id)
	if err != nil {
		return GroupResponse{}, err
	}

	return MapDbGroupToApiGroupResponse(newGroup), nil
}

func GetGroupById(db store.Store, ctx context.Context, groupId, userId string) (GroupResponse, error) {
	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupResponse{}, ErrGroupNotFound
		}
		return GroupResponse{}, err
	}

	return MapDbGroupToApiGroupResponse(groupDb), nil
}

// RenameGroup renames a group the caller owns. Owner-only; validates a non-empty
// name and maps a duplicate name to ErrGroupDuplicatedName.
func UpdateGroupInfo(db store.Store, ctx context.Context, groupId, ownerId, name, description string) (GroupResponse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GroupResponse{}, ErrGroupNameInvalid
	}
	description = strings.TrimSpace(description)

	group, err := db.GetGroupById(ctx, groupId, ownerId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupResponse{}, ErrGroupNotFound
		}
		return GroupResponse{}, err
	}

	if group.OwnerId != ownerId {
		return GroupResponse{}, ErrGroupNotOwnedByUser
	}

	if err := db.UpdateGroupInfo(ctx, groupId, name, description); err != nil {
		if errors.Is(err, store.ErrDuplicatedRecord) {
			return GroupResponse{}, ErrGroupDuplicatedName
		}
		return GroupResponse{}, err
	}

	group.Name = name
	group.Description = description
	return MapDbGroupToApiGroupResponse(group), nil
}

func AddUserToGroup(db store.Store, ctx context.Context, groupId, ownerId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, ownerId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	// Only the owner of the group can add users to it
	if group.OwnerId != ownerId {
		return ErrGroupNotOwnedByUser
	}

	err = db.AddUserToGroup(ctx, groupId, ownerId, userId)
	if err != nil {
		return err
	}

	_, err = users.UpdateUserGroup(db, ctx, userId, groupId)
	if err != nil {
		return err
	}

	return nil
}

// GetTitlesFromGroup returns one page of a group's titles.
//
// It does NOT check that the group exists or that the caller may see it: the
// caller must have established that first. The HTTP handler does, with
// groups.GroupExists — a single EXISTS whose 404 this function cannot improve
// on — so running the same check again here would only add a round trip to
// the endpoint. That is also why it takes no userId: an ignored one would
// imply a per-user scoping this function does not perform.
func GetTitlesFromGroup(
	db store.Store,
	ctx context.Context,
	groupId string,
	size, page int,
	orderBy string,
	watched *bool,
	ascending *bool,
	titleType *string,
) (generics.Page[GroupTitleDetail], error) {
	// API vocabulary -> title.type values; anything unrecognized means no
	// filter, matching the previous behavior.
	var titleTypes []string
	if titleType != nil {
		switch *titleType {
		case "serie":
			titleTypes = models.SeriesTitleTypes()
		case "movie":
			titleTypes = []string{"movie"}
		}
	}

	// App-level pagination normalization for the query itself, shared with
	// titles.GetPageOfTitles. The raw, caller-given size/page are deliberately
	// kept alongside: the "group holds nothing to page over" response below
	// reports them unnormalized — see the comment there.
	querySize, queryPage := config.NormalizePageParams(size, page)

	pageRows, total, err := db.GetGroupTitlesPage(ctx, groupId, watched, titleTypes, orderBy, ascending, querySize, queryPage)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	// Three distinct situations produce an empty page, and clients can tell
	// two shapes apart (`"Content":[]` vs `"Content":null`, plus whether
	// size/page are normalized), so the split is an observable API contract
	// (CONVENTIONS §5) that this function must keep:
	//
	//  1. the group holds no title entry matching the filters — an empty
	//     group, or a watched/titleType filter that matches none of its
	//     entries: `[]`, with the caller's raw size/page echoed back;
	//  2. every matching entry points at a title that is gone from the
	//     catalogue (group_titles has no FK to titles, so entries outlive
	//     deleted titles): `null`, with normalized size/page;
	//  3. the requested page is past the last one: `null`, with normalized
	//     size/page and a non-zero total.
	//
	// GetGroupTitlesPage inner-joins titles, so its total is 0 for both (1)
	// and (2) — hence the extra EXISTS, which counts orphaned entries and so
	// separates them. It runs only on this already-empty path, never on the
	// hot one. Cases (2) and (3) need no branch at all: falling through
	// leaves allTitlesDetails nil, which is exactly the `null` those two
	// return.
	if total == 0 {
		hasEntries, err := db.GroupHasTitleEntries(ctx, groupId, watched, titleTypes)
		if err != nil {
			return generics.Page[GroupTitleDetail]{}, err
		}
		if !hasEntries {
			return generics.Page[GroupTitleDetail]{
				TotalResults: 0,
				Size:         size,
				Page:         page,
				TotalPages:   0,
				Content:      []GroupTitleDetail{},
			}, nil
		}
	}

	pageTitleIds := make([]string, 0, len(pageRows))
	for _, row := range pageRows {
		pageTitleIds = append(pageTitleIds, row.Title.ID)
	}

	// Scoped to this group: GroupRatings must carry only the ratings this
	// group's own members left in this group, never another group's ratings on
	// the same title.
	groupRatings, err := ratings.GetRatingsBatch(db, ctx, pageTitleIds, groupId)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesDetails []GroupTitleDetail
	for _, row := range pageRows {
		detail := GroupTitleDetail{
			GroupRatings: groupRatings.Titles[row.Title.ID],
			Watched:      row.Item.Watched,
			WatchedAt:    row.Item.WatchedAt,
			AddedAt:      row.Item.AddedAt,
			UpdatedAt:    row.Item.UpdatedAt,
		}

		// Map seasons watched from database to API type
		if row.Item.SeasonsWatched != nil {
			seasonsWatched := make(SeasonsWatched)
			for seasonKey, seasonDb := range *row.Item.SeasonsWatched {
				seasonsWatched[seasonKey] = SeasonWatched{
					Watched:   seasonDb.Watched,
					WatchedAt: seasonDb.WatchedAt,
					AddedAt:   seasonDb.AddedAt,
					UpdatedAt: seasonDb.UpdatedAt,
				}
			}
			detail.SeasonsWatched = &seasonsWatched
		}

		detail.Title = titles.MapDbTitleToApiTitle(row.Title)
		// Episodes are loaded on demand via GET /titles/{id}/episodes;
		// keep the list payload light. Seasons summary is retained.
		detail.Title.Episodes = nil
		allTitlesDetails = append(allTitlesDetails, detail)
	}

	totalPages := int(math.Ceil(float64(total) / float64(querySize)))
	return generics.Page[GroupTitleDetail]{
		TotalResults: int(total),
		Size:         querySize,
		Page:         queryPage,
		TotalPages:   totalPages,
		Content:      allTitlesDetails,
	}, nil
}

func GetUsersFromGroup(db store.Store, ctx context.Context, groupId, userId string) ([]users.UserResponse, error) {
	usersFromGroup, err := db.GetUsersFromGroup(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return []users.UserResponse{}, ErrGroupNotFound
		}
		return []users.UserResponse{}, err
	}

	var usersResponse []users.UserResponse
	for _, user := range usersFromGroup {
		usersResponse = append(usersResponse, users.MapDbUserToApiUserResponse(user))
	}

	return usersResponse, nil
}

func AddTitleToGroup(db store.Store, ctx context.Context, groupId, titleId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	if _, exists := group.Titles[titleId]; exists {
		return ErrTitleAlreadyInGroup
	}

	err = db.AddNewGroupTitle(ctx, groupId, titleId)
	if err != nil {
		return err
	}
	return nil
}

// UpdateGroupTitleWatched updates the watched status of a title in a group.
//
// It routes to the appropriate handler based on whether a season is provided:
//   - updateGroupTitleWatchedForTVSeries: if season is provided (TV series case)
//   - updateGroupTitleWatchedForMovie: if season is not provided (movie case)
//
// Possible errors:
//   - ErrGroupNotFound: if the group is not found
//   - ErrTitleNotInGroup: if the title is not found in the group
//   - ErrInvalidSeasonValue: if season is provided and is less than or equal to zero
//   - ErrSeasonDoesNotExist: if season is provided but doesn't exist in the title
//   - ErrUpdatingWatchedAtWhenWatchedIsFalse: if trying to update watchedAt when watched is false
//
// Alongside the updated title it returns the WatchedChange the call produced —
// the addressed state before and after — since both halves are only knowable
// here, and the activity feed needs them to tell "marked watched" from "moved
// the date".
func UpdateGroupTitleWatched(
	db store.Store,
	ctx context.Context,
	groupId string,
	title titles.Title,
	userId string,
	watched *bool,
	watchedAt *generics.FlexibleDate,
	season *int,
) (GroupTitle, WatchedChange, error) {
	if season != nil {
		return updateGroupTitleWatchedForTVSeries(db, ctx, groupId, title, userId, watched, watchedAt, season)
	}

	return updateGroupTitleWatchedForMovie(db, ctx, groupId, title, userId, watched, watchedAt)
}

// updateGroupTitleWatchedForMovie handles watched status updates for movies.
//
// Steps performed by this method:
//  1. Validates that the title exists in the group.
//  2. Validates watchedAt update rules (cannot update watchedAt when watched is false).
//  3. Clears watchedAt if watched is set to false.
//  4. Updates the watched and watchedAt fields in the database.
//
// Possible errors:
//   - ErrGroupNotFound: if the group is not found
//   - ErrTitleNotInGroup: if the title is not found in the group
//   - ErrUpdatingWatchedAtWhenWatchedIsFalse: if trying to update watchedAt when watched is false
func updateGroupTitleWatchedForMovie(
	db store.Store,
	ctx context.Context,
	groupId string,
	title titles.Title,
	userId string,
	watched *bool,
	watchedAt *generics.FlexibleDate,
) (GroupTitle, WatchedChange, error) {
	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, WatchedChange{}, ErrGroupNotFound
		}
		return GroupTitle{}, WatchedChange{}, err
	}

	titleDb, exists := groupDb.Titles[title.Id]
	if !exists {
		return GroupTitle{}, WatchedChange{}, ErrTitleNotInGroup
	}

	// The row just read is the only chance to see the state the update is
	// about to overwrite, so capture it here rather than re-reading afterwards.
	previous := WatchedState{Watched: titleDb.Watched, WatchedAt: titleDb.WatchedAt}

	// Don't allow updating watchedAt if watched is set to false or when title is not watched
	watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
	watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
	if !titleDb.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
		return GroupTitle{}, WatchedChange{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
	}

	// If the request comes with the watched field set to false, clear the watchedAt field.
	// We must always pass a FlexibleDate with Time = nil to clear watchedAt in the database.
	if watched != nil && !*watched {
		watchedAt = &generics.FlexibleDate{Time: nil}
	}

	groupTitleItem, err := db.UpdateGroupTitleWatchedForMovie(ctx, groupId, title.Id, watched, watchedAt)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, WatchedChange{}, ErrTitleNotInGroup
		}
		return GroupTitle{}, WatchedChange{}, err
	}

	change := WatchedChange{
		Current:  WatchedState{Watched: groupTitleItem.Watched, WatchedAt: groupTitleItem.WatchedAt},
		Previous: previous,
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), change, nil
}

// updateGroupTitleWatchedForTVSeries handles watched status updates for TV series seasons.
//
// Steps performed by this method:
//  1. Validates that the season value is valid (greater than zero).
//  2. Validates that the title is a TV series type.
//  3. Validates that the season exists in the title's seasons list.
//  4. Validates watchedAt update rules for the specific season.
//  5. Clears watchedAt if watched is set to false.
//  6. Updates the seasonsWatched map for the specific season in the database.
//
// Possible errors:
//   - ErrGroupNotFound: if the group is not found
//   - ErrTitleNotInGroup: if the title is not found in the group
//   - ErrInvalidSeasonValue: if season is less than or equal to zero
//   - ErrSeasonDoesNotExist: if the season doesn't exist in the title
//   - ErrUpdatingWatchedAtWhenWatchedIsFalse: if trying to update watchedAt when season is not watched
func updateGroupTitleWatchedForTVSeries(
	db store.Store,
	ctx context.Context,
	groupId string,
	title titles.Title,
	userId string,
	watched *bool,
	watchedAt *generics.FlexibleDate,
	season *int,
) (GroupTitle, WatchedChange, error) {
	// 1. Validate season value
	if *season <= 0 {
		return GroupTitle{}, WatchedChange{}, ErrInvalidSeasonValue
	}

	// 2. Check if title is a TV series
	if title.Type != "tvSeries" && title.Type != "tvMiniSeries" {
		return GroupTitle{}, WatchedChange{}, ErrSeasonDoesNotExist
	}

	// 3. Check if the season exists in the title
	seasonAsString := strconv.Itoa(*season)
	seasonExists := false
	for _, s := range title.Seasons {
		if s.Season == seasonAsString {
			seasonExists = true
			break
		}
	}
	if !seasonExists {
		return GroupTitle{}, WatchedChange{}, ErrSeasonDoesNotExist
	}

	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, WatchedChange{}, ErrGroupNotFound
		}
		return GroupTitle{}, WatchedChange{}, err
	}

	titleDb, exists := groupDb.Titles[title.Id]
	if !exists {
		return GroupTitle{}, WatchedChange{}, ErrTitleNotInGroup
	}

	// The season row about to be overwritten. A season nobody has touched yet
	// has no row at all, which is the same story the feed tells as an unwatched
	// one: not watched, no date.
	previous := seasonWatchedState(titleDb.SeasonsWatched, seasonAsString)

	// 4. For TV series seasons, validate watchedAt rules
	if titleDb.SeasonsWatched != nil {
		existingSeason, hasSeason := (*titleDb.SeasonsWatched)[seasonAsString]
		if hasSeason {
			// Don't allow updating watchedAt if watched is set to false or when season is not watched
			watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
			watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
			if !existingSeason.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
				return GroupTitle{}, WatchedChange{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
			}
		}
	}

	// 5. If the request comes with the watched field set to false, clear the watchedAt field.
	if watched != nil && !*watched {
		watchedAt = &generics.FlexibleDate{Time: nil}
	}

	// 6. Update the season in the database
	groupTitleItem, err := db.UpdateGroupTitleWatchedForTVSeries(ctx, groupId, title.Id, watched, watchedAt, *season, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, WatchedChange{}, ErrTitleNotInGroup
		}
		return GroupTitle{}, WatchedChange{}, err
	}

	// The season's own resulting state, not the title's: the store recomputes
	// the top-level watched/watchedAt across every season, so those describe
	// the series as a whole and would misreport a single season's change.
	change := WatchedChange{
		Current:  seasonWatchedState(groupTitleItem.SeasonsWatched, seasonAsString),
		Previous: previous,
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), change, nil
}

// seasonWatchedState reads one season out of a title's per-season map. A nil
// map, or a season with no row in it, reads as not watched with no date — the
// state a season starts in before anyone records anything against it.
func seasonWatchedState(seasons *models.SeasonsWatched, season string) WatchedState {
	if seasons == nil {
		return WatchedState{}
	}
	item, ok := (*seasons)[season]
	if !ok {
		return WatchedState{}
	}
	return WatchedState{Watched: item.Watched, WatchedAt: item.WatchedAt}
}

func RemoveTitleFromGroup(db store.Store, ctx context.Context, groupId, titleId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	if _, exists := group.Titles[titleId]; !exists {
		return ErrTitleNotInGroup
	}

	err = db.RemoveTitleFromGroup(ctx, groupId, titleId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrTitleNotInGroup
		}
		return err
	}
	return nil
}

// SoftDeleteGroup marks a group deleted (owner only) and removes it from every
// member's group list. No cascade to titles/ratings/comments.
func SoftDeleteGroup(db store.Store, ctx context.Context, groupId, ownerId string) error {
	group, err := db.GetGroupById(ctx, groupId, ownerId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	if group.OwnerId != ownerId {
		return ErrGroupNotOwnedByUser
	}
	if err := db.SoftDeleteGroup(ctx, groupId); err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	for _, memberId := range group.Users {
		if err := db.RemoveGroupFromUser(ctx, memberId, groupId); err != nil {
			return err
		}
	}
	return nil
}

// LeaveGroup removes a non-owner member from a group (and the group from their
// group list). The owner cannot leave (must delete instead).
func LeaveGroup(db store.Store, ctx context.Context, groupId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	if group.OwnerId == userId {
		return ErrOwnerCannotLeaveGroup
	}
	if err := db.RemoveUserFromGroup(ctx, groupId, userId); err != nil {
		return err
	}
	return db.RemoveGroupFromUser(ctx, userId, groupId)
}

// GroupExists reports whether the group exists for the given user. Thin service
// passthrough so handlers reach the DB only through the service layer.
func GroupExists(db store.Store, ctx context.Context, groupId, userId string) (bool, error) {
	return db.GroupExists(ctx, groupId, userId)
}

// GroupContainsTitle reports whether the group (owned/shared with the user)
// contains the given title. Thin service passthrough.
func GroupContainsTitle(db store.Store, ctx context.Context, groupId, titleId, userId string) (bool, error) {
	return db.GroupContainsTitle(ctx, groupId, titleId, userId)
}
