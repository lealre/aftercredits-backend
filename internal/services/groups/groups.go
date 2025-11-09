package groups

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
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
	}
}
