package groups

import (
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
)

type Group struct {
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	OwnerId     string       `json:"ownerId"`
	Users       UsersIds     `json:"users"`
	Titles      []GroupTitle `json:"titles"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type UsersIds []string

type GroupTitle struct {
	Id             string          `json:"id"`
	Watched        bool            `json:"watched"`
	SeasonsWatched *SeasonsWatched `json:"seasonsWatched,omitempty"`
	AddedAt        time.Time       `json:"addedAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	WatchedAt      *time.Time      `json:"watchedAt,omitempty"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AddUserToGroupRequest struct {
	UserId string `json:"userId"`
}

type GroupResponse struct {
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	OwnerId     string       `json:"ownerId"`
	Users       UsersIds     `json:"users"`
	Titles      []GroupTitle `json:"titles"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type SeasonWatched struct {
	Watched   bool       `json:"watched"`
	WatchedAt *time.Time `json:"watchedAt,omitempty"`
	AddedAt   time.Time  `json:"addedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type SeasonsWatched map[string]SeasonWatched

type GroupTitleDetail struct {
	titles.Title
	GroupRatings   []ratings.Rating `json:"groupRatings"`
	SeasonsWatched *SeasonsWatched  `json:"seasonsWatched,omitempty"`
	Watched        bool             `json:"watched"`
	AddedAt        time.Time        `json:"addedAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
	WatchedAt      *time.Time       `json:"watchedAt,omitempty"`
}

type AddTitleToGroupRequest struct {
	URL     string `json:"url"`
	GroupId string `json:"groupId"`
}

// WatchedState is a watched flag with the date it was watched, when there is
// one.
type WatchedState struct {
	Watched   bool
	WatchedAt *time.Time
}

// WatchedChange is the before and after of one watched update, scoped to
// whatever the request addressed: the title as a whole for a movie, a single
// season for a series.
//
// It is returned alongside the updated GroupTitle because that value cannot
// answer for either half. The previous state is gone by the time the update
// returns, and for a season-scoped change the top-level Watched/WatchedAt on
// GroupTitle are recomputed across every season ("watched if any season is"),
// so they describe the title, not the season that was just changed.
//
// The service is where this comes from rather than a second read in the
// handler: UpdateGroupTitleWatched already loads the group title to validate
// the request, so the prior state is in hand and costs nothing to return.
type WatchedChange struct {
	Current  WatchedState
	Previous WatchedState
}

type UpdateGroupTitleWatchedRequest struct {
	TitleId   string                 `json:"titleId"`
	Season    *int                   `json:"season,omitempty"`
	Watched   *bool                  `json:"watched,omitempty"`
	WatchedAt *generics.FlexibleDate `json:"watchedAt,omitempty"`
}
