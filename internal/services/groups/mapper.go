package groups

import "github.com/lealre/movies-backend/internal/mongodb"

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

func MapDbGroupTitleToApiGroupTitle(title mongodb.GroupTitleItemDb) GroupTitle {
	watched := title.Watched
	if !watched {
		watched = false
	}

	groupTitle := GroupTitle{
		Id:        title.TitleId,
		Watched:   watched,
		AddedAt:   title.AddedAt,
		UpdatedAt: title.UpdatedAt,
		WatchedAt: title.WatchedAt,
	}

	// Map seasons watched from database to API type
	if title.SeasonsWatched != nil {
		seasonsWatched := make(SeasonsWatched)
		for seasonKey, seasonDb := range *title.SeasonsWatched {
			seasonsWatched[seasonKey] = SeasonWatched{
				Watched:   seasonDb.Watched,
				WatchedAt: seasonDb.WatchedAt,
				AddedAt:   seasonDb.AddedAt,
				UpdatedAt: seasonDb.UpdatedAt,
			}
		}
		groupTitle.SeasonsWatched = &seasonsWatched
	}

	return groupTitle
}
