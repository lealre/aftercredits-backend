package groups

import "time"

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
	Id        string     `json:"id"`
	Watched   bool       `json:"watched"`
	AddedAt   time.Time  `json:"addedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	WatchedAt *time.Time `json:"watchedAt,omitempty"`
}

type CreateGroupRequest struct {
	Name    string `json:"name"`
	OwnerId string `json:"ownerId"`
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
