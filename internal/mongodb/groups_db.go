package mongodb

import (
	"time"
)

// ----- Types for the database -----

type GroupDb struct {
	Id        string         `json:"id" bson:"_id"`
	OwnerId   string         `json:"ownerId" bson:"ownerId"`
	Users     UsersIds       `json:"users" bson:"users"`
	Titles    []GroupTitleDb `json:"titles" bson:"titles"`
	CreatedAt time.Time      `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt" bson:"updatedAt"`
}

type UsersIds []string

type GroupTitleDb struct {
	Id        string     `json:"id" bson:"_id"`
	Watched   bool       `json:"watched" bson:"watched"`
	AddedAt   time.Time  `json:"addedAt" bson:"addedAt"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updatedAt"`
	WatchedAt *time.Time `json:"watchedAt,omitempty" bson:"watchedAt,omitempty"`
}

// ----- Methods for the database -----
