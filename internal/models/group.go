package models

import "time"

// Group is the storage-neutral representation of a group (mirroring
// mongodb.GroupDb, without persistence tags).
type Group struct {
	Id          string
	Name        string
	Description string
	OwnerId     string
	Users       []string
	Titles      GroupTitles
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Deleted     bool
	DeletedAt   *time.Time
}

// GroupTitles is the storage-neutral representation of mongodb.GroupTitleDb,
// a group's titles keyed by title id.
type GroupTitles map[string]GroupTitleItem

// GroupTitleItem is the storage-neutral representation of
// mongodb.GroupTitleItemDb.
type GroupTitleItem struct {
	TitleId        string
	TitleType      string
	SeasonsWatched *SeasonsWatched
	Watched        bool
	AddedAt        time.Time
	UpdatedAt      time.Time
	WatchedAt      *time.Time
}

// SeasonsWatched is the storage-neutral representation of
// mongodb.SeasonWatchedDb, a title's per-season watched state keyed by
// season number (as a string).
type SeasonsWatched map[string]SeasonWatchedItem

// SeasonWatchedItem is the storage-neutral representation of
// mongodb.SeasonWatchedItemDb.
type SeasonWatchedItem struct {
	Watched   bool
	WatchedAt *time.Time
	AddedAt   time.Time
	UpdatedAt time.Time
}
