package mongodb

import "github.com/lealre/movies-backend/internal/models"

// groupDbToModel converts the mongo-specific GroupDb into the storage-neutral
// models.Group used by the service layer.
func groupDbToModel(g GroupDb) models.Group {
	return models.Group{
		Id:          g.Id,
		Name:        g.Name,
		Description: g.Description,
		OwnerId:     g.OwnerId,
		Users:       []string(g.Users),
		Titles:      groupTitlesDbToModel(g.Titles),
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
		Deleted:     g.Deleted,
		DeletedAt:   g.DeletedAt,
	}
}

// groupModelToDb converts a storage-neutral models.Group back into the
// mongo-specific GroupDb used at the persistence boundary.
func groupModelToDb(g models.Group) GroupDb {
	return GroupDb{
		Id:          g.Id,
		Name:        g.Name,
		Description: g.Description,
		OwnerId:     g.OwnerId,
		Users:       UsersIds(g.Users),
		Titles:      groupTitlesModelToDb(g.Titles),
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
		Deleted:     g.Deleted,
		DeletedAt:   g.DeletedAt,
	}
}

// ----- Titles map -----

func groupTitleItemDbToModel(t GroupTitleItemDb) models.GroupTitleItem {
	return models.GroupTitleItem{
		TitleId:        t.TitleId,
		TitleType:      t.TitleType,
		SeasonsWatched: seasonsWatchedDbToModel(t.SeasonsWatched),
		Watched:        t.Watched,
		AddedAt:        t.AddedAt,
		UpdatedAt:      t.UpdatedAt,
		WatchedAt:      t.WatchedAt,
	}
}

func groupTitleItemModelToDb(t models.GroupTitleItem) GroupTitleItemDb {
	return GroupTitleItemDb{
		TitleId:        t.TitleId,
		TitleType:      t.TitleType,
		SeasonsWatched: seasonsWatchedModelToDb(t.SeasonsWatched),
		Watched:        t.Watched,
		AddedAt:        t.AddedAt,
		UpdatedAt:      t.UpdatedAt,
		WatchedAt:      t.WatchedAt,
	}
}

func groupTitlesDbToModel(t GroupTitleDb) models.GroupTitles {
	if t == nil {
		return nil
	}
	out := make(models.GroupTitles, len(t))
	for k, v := range t {
		out[string(k)] = groupTitleItemDbToModel(v)
	}
	return out
}

func groupTitlesModelToDb(t models.GroupTitles) GroupTitleDb {
	if t == nil {
		return nil
	}
	out := make(GroupTitleDb, len(t))
	for k, v := range t {
		out[TitleId(k)] = groupTitleItemModelToDb(v)
	}
	return out
}

// ----- SeasonsWatched map -----

func seasonWatchedItemDbToModel(s SeasonWatchedItemDb) models.SeasonWatchedItem {
	return models.SeasonWatchedItem{
		Watched:   s.Watched,
		WatchedAt: s.WatchedAt,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonWatchedItemModelToDb(s models.SeasonWatchedItem) SeasonWatchedItemDb {
	return SeasonWatchedItemDb{
		Watched:   s.Watched,
		WatchedAt: s.WatchedAt,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonsWatchedDbToModel(s *SeasonWatchedDb) *models.SeasonsWatched {
	if s == nil {
		return nil
	}
	out := make(models.SeasonsWatched, len(*s))
	for k, v := range *s {
		out[k] = seasonWatchedItemDbToModel(v)
	}
	return &out
}

func seasonsWatchedModelToDb(s *models.SeasonsWatched) *SeasonWatchedDb {
	if s == nil {
		return nil
	}
	out := make(SeasonWatchedDb, len(*s))
	for k, v := range *s {
		out[k] = seasonWatchedItemModelToDb(v)
	}
	return &out
}
