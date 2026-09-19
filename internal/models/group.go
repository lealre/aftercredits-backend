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
	SeasonsWatched *SeasonsWatched
	Watched        bool
	AddedAt        time.Time
	UpdatedAt      time.Time
	WatchedAt      *time.Time
	// AddedBy is nil when no author was recorded: the entry predates the column,
	// or the member who added it has since been deleted.
	AddedBy *TitleAuthor
	// WatchedMarkedBy is who set the CURRENT watched state, not who watched it —
	// watched is a fact about the group. Nil when unwatched, when the marking
	// predates the column, or when that account was deleted.
	WatchedMarkedBy *TitleAuthor
	// WatchedMarkedAt is WHEN that marking happened, which is not WatchedAt.
	// WatchedAt is the date the group says it saw the film, chosen by a member
	// and often in the past; this is the system time of the action.
	WatchedMarkedAt *time.Time
}

// TitleAuthor identifies the member who added a title to a group. The username
// is joined at read time rather than stored alongside the id, so a rename is
// reflected everywhere instead of leaving old rows naming someone who no
// longer goes by that.
type TitleAuthor struct {
	Id       string
	Username string
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
