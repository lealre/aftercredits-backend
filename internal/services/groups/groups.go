package groups

import (
	"context"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/services/users"
)

func CreateGroup(db *mongodb.DB, ctx context.Context, req CreateGroupRequest) (GroupResponse, error) {
	group := mongodb.GroupDb{
		Name:    req.Name,
		OwnerId: req.OwnerId,
		Users:   []string{req.OwnerId},
		Titles:  []mongodb.GroupTitleDb{},
	}

	newGroup, err := db.CreateGroup(ctx, group)
	if err != nil {
		return GroupResponse{}, err
	}

	return MapDbGroupToApiGroupResponse(newGroup), nil
}

func MapDbGroupToApiGroupResponse(group mongodb.GroupDb) GroupResponse {
	groupResponse := GroupResponse{
		Id:        group.Id,
		Name:      group.Name,
		OwnerId:   group.OwnerId,
		Users:     UsersIds(group.Users),
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}

	for _, title := range group.Titles {
		groupResponse.Titles = append(groupResponse.Titles, MapDbGroupTitleToApiGroupTitle(title))
	}

	return groupResponse
}

func MapDbGroupTitleToApiGroupTitle(title mongodb.GroupTitleDb) GroupTitle {
	watched := title.Watched
	if !watched {
		watched = false
	}

	return GroupTitle{
		Id:        title.Id,
		Watched:   watched,
		AddedAt:   title.AddedAt,
		UpdatedAt: title.UpdatedAt,
		WatchedAt: title.WatchedAt,
	}
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
	group, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return generics.Page[GroupTitleDetail]{}, err
	}

	var allTitlesIds []string
	var titleGroupMap map[string]mongodb.GroupTitleDb = make(map[string]mongodb.GroupTitleDb)
	for _, title := range group.Titles {
		titleGroupMap[title.Id] = title
		allTitlesIds = append(allTitlesIds, title.Id)
	}

	titles, err := titles.GetPageOfTitles(db, ctx, size, page, orderBy, watched, ascending, allTitlesIds)
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
	err := db.AddNewGroupTitle(ctx, groupId, titleId)
	if err != nil {
		return err
	}
	return nil
}

func UpdateGroupTitleWatched(db *mongodb.DB, ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (GroupTitle, error) {
	groupTitle, err := db.UpdateGroupTitleWatched(ctx, groupId, titleId, watched, watchedAt)
	if err != nil {
		return GroupTitle{}, err
	}
	return MapDbGroupTitleToApiGroupTitle(*groupTitle), nil
}
