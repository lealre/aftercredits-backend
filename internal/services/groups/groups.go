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

func GetTitlesFromGroup(
	db store.Store,
	ctx context.Context,
	groupId, userId string,
	size, page int,
	orderBy string,
	watched *bool,
	ascending *bool,
	titleType *string,
) (generics.Page[GroupTitleDetail], error) {
	ok, err := db.GroupExists(ctx, groupId, userId)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}
	if !ok {
		return generics.Page[GroupTitleDetail]{}, ErrGroupNotFound
	}

	// API vocabulary -> title.type values; anything unrecognized means no
	// filter, matching the previous behavior.
	var titleTypes []string
	if titleType != nil {
		switch *titleType {
		case "serie":
			titleTypes = []string{"tvSeries", "tvMiniSeries"}
		case "movie":
			titleTypes = []string{"movie"}
		}
	}

	// App-level pagination normalization for the query itself, mirroring
	// titles.GetPageOfTitles. The raw, caller-given size/page are kept for the
	// zero-total response below, matching the previous early-return shape.
	querySize := size
	if querySize <= 0 {
		querySize = config.DefaultPageSize()
	}
	if maxSize := config.MaxPageSize(); querySize > maxSize {
		querySize = maxSize
	}
	queryPage := page
	if queryPage <= 0 {
		queryPage = 1
	}

	pageRows, total, err := db.GetGroupTitlesPage(ctx, groupId, watched, titleTypes, orderBy, ascending, querySize, queryPage)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	// Covers a plain empty group as well as a watched/titleType filter that
	// matches nothing.
	if total == 0 {
		return generics.Page[GroupTitleDetail]{
			TotalResults: 0,
			Size:         size,
			Page:         page,
			TotalPages:   0,
			Content:      []GroupTitleDetail{},
		}, nil
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
func UpdateGroupTitleWatched(
	db store.Store,
	ctx context.Context,
	groupId string,
	title titles.Title,
	userId string,
	watched *bool,
	watchedAt *generics.FlexibleDate,
	season *int,
) (GroupTitle, error) {
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
) (GroupTitle, error) {
	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, ErrGroupNotFound
		}
		return GroupTitle{}, err
	}

	titleDb, exists := groupDb.Titles[title.Id]
	if !exists {
		return GroupTitle{}, ErrTitleNotInGroup
	}

	// Don't allow updating watchedAt if watched is set to false or when title is not watched
	watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
	watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
	if !titleDb.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
		return GroupTitle{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
	}

	// If the request comes with the watched field set to false, clear the watchedAt field.
	// We must always pass a FlexibleDate with Time = nil to clear watchedAt in the database.
	if watched != nil && !*watched {
		watchedAt = &generics.FlexibleDate{Time: nil}
	}

	groupTitleItem, err := db.UpdateGroupTitleWatchedForMovie(ctx, groupId, title.Id, watched, watchedAt)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, ErrTitleNotInGroup
		}
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), nil
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
) (GroupTitle, error) {
	// 1. Validate season value
	if *season <= 0 {
		return GroupTitle{}, ErrInvalidSeasonValue
	}

	// 2. Check if title is a TV series
	if title.Type != "tvSeries" && title.Type != "tvMiniSeries" {
		return GroupTitle{}, ErrSeasonDoesNotExist
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
		return GroupTitle{}, ErrSeasonDoesNotExist
	}

	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return GroupTitle{}, ErrGroupNotFound
		}
		return GroupTitle{}, err
	}

	titleDb, exists := groupDb.Titles[title.Id]
	if !exists {
		return GroupTitle{}, ErrTitleNotInGroup
	}

	// 4. For TV series seasons, validate watchedAt rules
	if titleDb.SeasonsWatched != nil {
		existingSeason, hasSeason := (*titleDb.SeasonsWatched)[seasonAsString]
		if hasSeason {
			// Don't allow updating watchedAt if watched is set to false or when season is not watched
			watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
			watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
			if !existingSeason.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
				return GroupTitle{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
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
			return GroupTitle{}, ErrTitleNotInGroup
		}
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), nil
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
