package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ----- Types for the database -----

type GroupDb struct {
	Id        string         `json:"id" bson:"_id"`
	Name      string         `json:"name" bson:"name"`
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

func (db *DB) CreateGroup(ctx context.Context, group GroupDb) (GroupDb, error) {
	coll := db.Collection(GroupsCollection)

	group.Id = primitive.NewObjectID().Hex()
	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	_, err := coll.InsertOne(ctx, group)
	if err != nil {
		return GroupDb{}, err
	}

	return group, nil
}
