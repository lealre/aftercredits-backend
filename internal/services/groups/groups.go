package groups

import (
	"context"
	"errors"
	"sort"
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

func UpdateGroupTitleWatched(
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
		return GroupTitle{}, err
	}

	// Don't allow updating watchedAt if watched is set to false or when title is not watched
	watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
	watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
	titleDb, exists := groupDb.Titles[mongodb.TitleId(title.Id)]
	if !exists {
		return GroupTitle{}, ErrTitleNotInGroup
	}

	if !titleDb.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
		return GroupTitle{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
	}

	// If the request comes with the watched field set to false, clear the watchedAt field.
	// We must always pass a FlexibleDate with Time = nil to clear watchedAt in the database.
	if watched != nil && !*watched {
		watchedAt = &generics.FlexibleDate{Time: nil}
	}

	groupTitleItem, err := db.UpdateGroupTitleWatched(ctx, groupId, title.Id, watched, watchedAt)
	if err != nil {
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitleItem), nil
}

// func updateTVSeriesTitleWatched(
// 	db *mongodb.DB,
// 	ctx context.Context,
// 	groupId, userId string,
// 	title titles.Title,
// 	watched *bool,
// 	watchedAt *generics.FlexibleDate,
// 	season *int,
// ) (GroupTitle, error) {

// 	// 1. Validate season value input(empty, negative or zero)
// 	if season == nil || *season <= 0 {
// 		return GroupTitle{}, ErrInvalidSeasonValue
// 	}

// 	seasonAsString := strconv.Itoa(*season)

// 	// 2. Check if the title has the season field
// 	for _, season := range title.Seasons {
// 		if season.Season == seasonAsString {
// 			break
// 		}
// 		return GroupTitle{}, ErrSeasonDoesNotExist
// 	}

// 	// 3. Check
// 	return GroupTitle{}, nil
// }

func RemoveTitleFromGroup(db *mongodb.DB, ctx context.Context, groupId, titleId, userId string) error {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		return err
	}

	if _, exists := group.Titles[mongodb.TitleId(titleId)]; !exists {
		return ErrTitleNotInGroup
	}

	return db.RemoveTitleFromGroup(ctx, groupId, titleId, userId)
}
