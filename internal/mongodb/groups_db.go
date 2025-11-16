package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Id        string     `json:"id" bson:"titleId"`
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

func (db *DB) GroupExists(ctx context.Context, id string) (bool, error) {
	coll := db.Collection(GroupsCollection)

	// Only ask MongoDB for the _id field
	opts := options.FindOne().SetProjection(bson.M{"_id": 1})

	err := coll.FindOne(ctx, bson.M{"_id": id}, opts).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (db *DB) GetGroupById(ctx context.Context, id string) (GroupDb, error) {
	coll := db.Collection(GroupsCollection)

	var group GroupDb
	err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&group)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return GroupDb{}, ErrRecordNotFound
		}
		return GroupDb{}, err
	}
	return group, nil
}

func (db *DB) GetUsersFromGroup(ctx context.Context, groupId string) ([]UserDb, error) {
	// First, get the group to find user IDs
	group, err := db.GetGroupById(ctx, groupId)
	if err != nil {
		return []UserDb{}, err
	}

	if len(group.Users) == 0 {
		return []UserDb{}, nil
	}

	// Query users collection with only _id and name fields
	usersColl := db.Collection(UsersCollection)
	filter := bson.M{"_id": bson.M{"$in": group.Users}}
	opts := options.Find().SetProjection(bson.M{
		"_id":  1,
		"name": 1,
	})

	cursor, err := usersColl.Find(ctx, filter, opts)
	if err != nil {
		return []UserDb{}, err
	}
	defer cursor.Close(ctx)

	var users []UserDb
	if err := cursor.All(ctx, &users); err != nil {
		return []UserDb{}, err
	}

	return users, nil
}

func (db *DB) UpdateGroup(ctx context.Context, group GroupDb) (GroupDb, error) {
	coll := db.Collection(GroupsCollection)

	_, err := coll.UpdateOne(ctx, bson.M{"_id": group.Id}, bson.M{"$set": group})
	if err != nil {
		return GroupDb{}, err
	}
	return group, nil
}

func (db *DB) AddNewGroupTitle(ctx context.Context, groupId string, titleId string) error {
	coll := db.Collection(GroupsCollection)

	_, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": groupId},
		bson.M{"$push": bson.M{"titles": GroupTitleDb{Id: titleId, Watched: false, AddedAt: time.Now(), UpdatedAt: time.Now()}}},
	)
	if err != nil {
		return err
	}
	return nil
}
