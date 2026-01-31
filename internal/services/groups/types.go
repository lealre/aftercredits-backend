package groups

import (
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/lealre/movies-backend/internal/services/titles"
)

type Group struct {
	Id        string       `json:"id"`
	Name      string       `json:"name"`
	OwnerId   string       `json:"ownerId"`
	Users     UsersIds     `json:"users"`
	Titles    []GroupTitle `json:"titles"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
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
	Name string `json:"name"`
}

type AddUserToGroupRequest struct {
	UserId string `json:"userId"`
}

type GroupResponse struct {
	Id        string       `json:"id"`
	Name      string       `json:"name"`
	OwnerId   string       `json:"ownerId"`
	Users     UsersIds     `json:"users"`
	Titles    []GroupTitle `json:"titles"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
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

type UpdateGroupTitleWatchedRequest struct {
	TitleId   string                 `json:"titleId"`
	Season    *int                   `json:"season,omitempty"`
	Watched   *bool                  `json:"watched,omitempty"`
	WatchedAt *generics.FlexibleDate `json:"watchedAt,omitempty"`
}
