package groups

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
)

var ErrTitleAlreadyInGroup = errors.New("title is already in group")
var ErrTitleNotInGroup = errors.New("title not found in group")
var ErrUpdatingWatchedAtWhenWatchedIsFalse = errors.New("cannot update watchedAt when watched is set to false")

func CreateGroup(db *mongodb.DB, ctx context.Context, req CreateGroupRequest, userId string) (GroupResponse, error) {
	group := mongodb.GroupDb{
		Name:    req.Name,
		OwnerId: userId,
		Users:   []string{userId},
		Titles:  []mongodb.GroupTitleDb{},
	}

	newGroup, err := db.CreateGroup(ctx, group)
	if err != nil {
		return GroupResponse{}, err
	}

	return MapDbGroupToApiGroupResponse(newGroup), nil
}

func AddUserToGroup(db *mongodb.DB, ctx context.Context, groupId, userId string) error {
	return db.AddUserToGroup(ctx, groupId, userId)
}

func GetTitlesFromGroup(
	db *mongodb.DB,
	ctx context.Context,
	groupId string,
	size, page int,
	orderBy string,
	watched *bool,
	ascending *bool,
) (generics.Page[GroupTitleDetail], error) {
	currentUser := auth.GetUserFromContext(ctx)

	group, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesIds []string
	var titleGroupMap map[string]mongodb.GroupTitleDb = make(map[string]mongodb.GroupTitleDb)
	for _, title := range group.Titles {

		if watched != nil && title.Watched != *watched {
			continue
		}

		titleGroupMap[title.Id] = title
		allTitlesIds = append(allTitlesIds, title.Id)
	}

	if len(allTitlesIds) > 1 && (orderBy == "watchedAt" || orderBy == "addedAt") {
		isAscending := true
		if ascending != nil {
			isAscending = *ascending
		}

		// If its a group field sorting, we must sort on the ids order of the group titles.
		// Later in GetPageOfTitles, it will mantain the order of the ids.
		if orderBy == "addedAt" || orderBy == "watchedAt" {
			getOrderValue := func(title mongodb.GroupTitleDb) (timeValue *time.Time) {
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

	ratings, err := ratings.GetRatingsBatch(db, ctx, allTitlesIds, currentUser.Id)
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

func GetUsersFromGroup(db *mongodb.DB, ctx context.Context, groupId string) ([]users.UserResponse, error) {
	usersDb, err := db.GetUsersFromGroup(ctx, groupId)
	if err != nil {
		return []users.UserResponse{}, err
	}

	var usersResponse []users.UserResponse
	for _, userDb := range usersDb {
		usersResponse = append(usersResponse, users.MapDbUserToApiUserResponse(userDb))
	}

	return usersResponse, nil
}

func AddTitleToGroup(db *mongodb.DB, ctx context.Context, groupId string, titleId string) error {
	group, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return err
	}

	for _, title := range group.Titles {
		if title.Id == titleId {
			return ErrTitleAlreadyInGroup
		}
	}

	err = db.AddNewGroupTitle(ctx, groupId, titleId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateGroupTitleWatched(db *mongodb.DB, ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (GroupTitle, error) {
	groupDb, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return GroupTitle{}, err
	}

	// Don't allow updating watchedAt if watched is set to false or when title is not watched
	watchedAtUpdateNotAllowedFalse := watchedAt != nil && watchedAt.Time != nil && watched != nil && !*watched
	watchedAtUpdateNotAllowedNil := watchedAt != nil && watchedAt.Time != nil && watched == nil
	for _, title := range groupDb.Titles {
		if title.Id != titleId {
			continue
		}

		if !title.Watched && (watchedAtUpdateNotAllowedFalse || watchedAtUpdateNotAllowedNil) {
			return GroupTitle{}, ErrUpdatingWatchedAtWhenWatchedIsFalse
		}
		break
	}

	// If the request comes with the watched field set to false, clear the watchedAt field.
	// We must always pass a FlexibleDate with Time = nil to clear watchedAt in the database.
	if watched != nil && !*watched {
		watchedAt = &generics.FlexibleDate{Time: nil}
	}

	groupTitle, err := db.UpdateGroupTitleWatched(ctx, groupId, titleId, watched, watchedAt)
	if err != nil {
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitle), nil
}

func RemoveTitleFromGroup(db *mongodb.DB, ctx context.Context, groupId string, titleId string) error {
	group, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return err
	}

	// Check if the title exists in the group
	found := false
	for _, title := range group.Titles {
		if title.Id == titleId {
			found = true
			break
		}
	}

	if !found {
		return ErrTitleNotInGroup
	}

	return db.RemoveTitleFromGroup(ctx, groupId, titleId)
}
