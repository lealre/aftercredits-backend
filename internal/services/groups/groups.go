package groups

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

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
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return generics.Page[GroupTitleDetail]{}, ErrGroupNotFound
		}
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesIds []string
	var titleGroupMap map[string]models.GroupTitleItem = make(map[string]models.GroupTitleItem)
	for _, title := range group.Titles {

		if watched != nil && title.Watched != *watched {
			continue
		}

		titleGroupMap[title.TitleId] = title
		allTitlesIds = append(allTitlesIds, title.TitleId)
	}

	// Filter by titleType if specified
	if titleType != nil && len(allTitlesIds) > 0 {
		// Fetch title types from database with lightweight projection
		titleTypes, err := db.GetTitleTypes(ctx, allTitlesIds)
		if err != nil {
			return generics.Page[GroupTitleDetail]{}, err
		}

		// Filter allTitlesIds based on titleType
		filteredTitlesIds := []string{}
		filteredTitleGroupMap := make(map[string]models.GroupTitleItem)

		for _, titleId := range allTitlesIds {
			titleTypeValue, exists := titleTypes[titleId]
			if !exists {
				// Skip titles that don't exist in database
				continue
			}

			shouldInclude := false
			if *titleType == "serie" {
				// Include tvSeries or tvMiniSeries
				shouldInclude = titleTypeValue == "tvSeries" || titleTypeValue == "tvMiniSeries"
			} else if *titleType == "movie" {
				// Include only movies
				shouldInclude = titleTypeValue == "movie"
			} else {
				// Invalid titleType value, include all (same as no filter)
				shouldInclude = true
			}

			if shouldInclude {
				filteredTitlesIds = append(filteredTitlesIds, titleId)
				filteredTitleGroupMap[titleId] = titleGroupMap[titleId]
			}
		}

		allTitlesIds = filteredTitlesIds
		titleGroupMap = filteredTitleGroupMap
	}

	// Check this after the watched/unwatched filter, to include that case as well
	if len(allTitlesIds) == 0 {
		return generics.Page[GroupTitleDetail]{
			TotalResults: 0,
			Size:         size,
			Page:         page,
			TotalPages:   0,
			Content:      []GroupTitleDetail{},
		}, nil
	}

	if len(allTitlesIds) > 1 && (orderBy == "watched" || orderBy == "watchedAt" || orderBy == "addedAt") {
		isAscending := true
		if ascending != nil {
			isAscending = *ascending
		}

		// If its a group field sorting, we must sort on the ids order of the group titles.
		// Later in GetPageOfTitles, it will mantain the order of the ids.
		//
		// Every branch here must impose a total order. allTitlesIds is built by
		// ranging group.Titles, which is a map, so its starting order differs
		// between requests; without a tie-break on the id the same query would
		// return a different page each time.
		if orderBy == "watched" {
			sort.SliceStable(allTitlesIds, func(i, j int) bool {
				left := titleGroupMap[allTitlesIds[i]]
				right := titleGroupMap[allTitlesIds[j]]

				if left.Watched == right.Watched {
					return allTitlesIds[i] < allTitlesIds[j]
				}
				// Ascending puts unwatched first, matching ORDER BY watched ASC.
				if isAscending {
					return !left.Watched
				}
				return left.Watched
			})
		}

		if orderBy == "addedAt" || orderBy == "watchedAt" {
			getOrderValue := func(title models.GroupTitleItem) (timeValue *time.Time) {
				if orderBy == "watchedAt" {
					return title.WatchedAt
				}
				return &title.AddedAt
			}

			sort.SliceStable(allTitlesIds, func(i, j int) bool {
				left := titleGroupMap[allTitlesIds[i]]
				right := titleGroupMap[allTitlesIds[j]]

				leftTime := getOrderValue(left)
				rightTime := getOrderValue(right)

				switch {
				case leftTime == nil && rightTime == nil:
					return allTitlesIds[i] < allTitlesIds[j]
				case leftTime == nil:
					return false
				case rightTime == nil:
					return true
				case leftTime.Equal(*rightTime):
					return allTitlesIds[i] < allTitlesIds[j]
				default:
					if isAscending {
						return leftTime.Before(*rightTime)
					}
					return leftTime.After(*rightTime)
				}
			})
		}
	}

	titles, err := titles.GetPageOfTitles(db, ctx, size, page, orderBy, ascending, allTitlesIds)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	// Only the titles on this page are read back below, so fetch ratings for
	// those ids rather than for every title in the group.
	pageTitleIds := make([]string, 0, len(titles.Content))
	for _, title := range titles.Content {
		pageTitleIds = append(pageTitleIds, title.Id)
	}

	ratings, err := ratings.GetRatingsBatch(db, ctx, pageTitleIds)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesDetails []GroupTitleDetail
	for _, title := range titles.Content {
		groupTitle := titleGroupMap[title.Id]
		detail := GroupTitleDetail{
			GroupRatings: ratings.Titles[title.Id],
			Watched:      groupTitle.Watched,
			WatchedAt:    groupTitle.WatchedAt,
			AddedAt:      groupTitle.AddedAt,
			UpdatedAt:    groupTitle.UpdatedAt,
		}

		// Map seasons watched from database to API type
		if groupTitle.SeasonsWatched != nil {
			seasonsWatched := make(SeasonsWatched)
			for seasonKey, seasonDb := range *groupTitle.SeasonsWatched {
				seasonsWatched[seasonKey] = SeasonWatched{
					Watched:   seasonDb.Watched,
					WatchedAt: seasonDb.WatchedAt,
					AddedAt:   seasonDb.AddedAt,
					UpdatedAt: seasonDb.UpdatedAt,
				}
			}
			detail.SeasonsWatched = &seasonsWatched
		}

		detail.Title = title
		// Episodes are loaded on demand via GET /titles/{id}/episodes;
		// keep the list payload light. Seasons summary is retained.
		detail.Title.Episodes = nil
		allTitlesDetails = append(allTitlesDetails, detail)
	}

	return generics.Page[GroupTitleDetail]{
		TotalResults: titles.TotalResults,
		Size:         titles.Size,
		Page:         titles.Page,
		TotalPages:   titles.TotalPages,
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
