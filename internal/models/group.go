package models

import "time"

// Group is the storage-neutral representation of a group, carrying no
// persistence tags.
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

// GroupTitles is a group's titles keyed by title id.
type GroupTitles map[string]GroupTitleItem

// GroupTitleItem is one title's entry within a group.
type GroupTitleItem struct {
	TitleId        string
	TitleType      string
	SeasonsWatched *SeasonsWatched
	Watched        bool
	AddedAt        time.Time
	UpdatedAt      time.Time
	WatchedAt      *time.Time
}

// SeasonsWatched is a title's per-season watched state keyed by season number
// (as a string).
type SeasonsWatched map[string]SeasonWatchedItem

// SeasonWatchedItem is the watched state of a single season.
type SeasonWatchedItem struct {
	Watched   bool
	WatchedAt *time.Time
	AddedAt   time.Time
	UpdatedAt time.Time
}

// GroupPagedTitle is one row of a group's paged titles listing: the full
// title plus this group's watch-state for it (seasons included).
type GroupPagedTitle struct {
	Title Title
	Item  GroupTitleItem
}
