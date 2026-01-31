package groups

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
)

func CreateGroup(db *mongodb.DB, ctx context.Context, req CreateGroupRequest, userId string) (GroupResponse, error) {

	if strings.TrimSpace(req.Name) == "" {
		return GroupResponse{}, ErrGroupNameInvalid
	}

	group := mongodb.GroupDb{
		Name:    req.Name,
		OwnerId: userId,
		Users:   []string{userId},
		Titles:  mongodb.GroupTitleDb{},
	}

	newGroup, err := db.CreateGroup(ctx, group)
	if err != nil {
		if errors.Is(err, mongodb.ErrDuplicatedRecord) {
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

func GetGroupById(db *mongodb.DB, ctx context.Context, groupId, userId string) (GroupResponse, error) {
	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return GroupResponse{}, ErrGroupNotFound
		}
		return GroupResponse{}, err
	}

	return MapDbGroupToApiGroupResponse(groupDb), nil
}

func AddUserToGroup(db *mongodb.DB, ctx context.Context, groupId, ownerId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, ownerId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
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
	db *mongodb.DB,
	ctx context.Context,
	groupId, userId string,
	size, page int,
	orderBy string,
	watched *bool,
	ascending *bool,
) (generics.Page[GroupTitleDetail], error) {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return generics.Page[GroupTitleDetail]{}, ErrGroupNotFound
		}
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesIds []string
	var titleGroupMap map[string]mongodb.GroupTitleItemDb = make(map[string]mongodb.GroupTitleItemDb)
	for _, title := range group.Titles {

		if watched != nil && title.Watched != *watched {
			continue
		}

		titleGroupMap[title.TitleId] = title
		allTitlesIds = append(allTitlesIds, title.TitleId)
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

	if len(allTitlesIds) > 1 && (orderBy == "watchedAt" || orderBy == "addedAt") {
		isAscending := true
		if ascending != nil {
			isAscending = *ascending
		}

		// If its a group field sorting, we must sort on the ids order of the group titles.
		// Later in GetPageOfTitles, it will mantain the order of the ids.
		if orderBy == "addedAt" || orderBy == "watchedAt" {
			getOrderValue := func(title mongodb.GroupTitleItemDb) (timeValue *time.Time) {
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

	ratings, err := ratings.GetRatingsBatch(db, ctx, allTitlesIds)
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

func GetUsersFromGroup(db *mongodb.DB, ctx context.Context, groupId, userId string) ([]users.UserResponse, error) {
	usersDb, err := db.GetUsersFromGroup(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return []users.UserResponse{}, ErrGroupNotFound
		}
		return []users.UserResponse{}, err
	}

	var usersResponse []users.UserResponse
	for _, userDb := range usersDb {
		usersResponse = append(usersResponse, users.MapDbUserToApiUserResponse(userDb))
	}

	return usersResponse, nil
}

func AddTitleToGroup(db *mongodb.DB, ctx context.Context, groupId, titleId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	if _, exists := group.Titles[mongodb.TitleId(titleId)]; exists {
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
	db *mongodb.DB,
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
	db *mongodb.DB,
	ctx context.Context,
	groupId string,
	title titles.Title,
	userId string,
	watched *bool,
	watchedAt *generics.FlexibleDate,
) (GroupTitle, error) {
	groupDb, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return GroupTitle{}, ErrGroupNotFound
		}
		return GroupTitle{}, err
	}

	titleDb, exists := groupDb.Titles[mongodb.TitleId(title.Id)]
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
		if errors.Is(err, mongodb.ErrRecordNotFound) {
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
	db *mongodb.DB,
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
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return GroupTitle{}, ErrGroupNotFound
		}
		return GroupTitle{}, err
	}

	titleDb, exists := groupDb.Titles[mongodb.TitleId(title.Id)]
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
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return GroupTitle{}, ErrTitleNotInGroup
		}
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), nil
}

func RemoveTitleFromGroup(db *mongodb.DB, ctx context.Context, groupId, titleId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}

	if _, exists := group.Titles[mongodb.TitleId(titleId)]; !exists {
		return ErrTitleNotInGroup
	}

	err = db.RemoveTitleFromGroup(ctx, groupId, titleId, userId)
	if err != nil {
		if errors.Is(err, mongodb.ErrRecordNotFound) {
			return ErrTitleNotInGroup
		}
		return err
	}
	return nil
}
