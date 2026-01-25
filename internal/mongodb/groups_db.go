package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ----- Types for the database -----

type GroupDb struct {
	Id        string       `json:"id" bson:"_id"`
	Name      string       `json:"name" bson:"name"`
	OwnerId   string       `json:"ownerId" bson:"ownerId"`
	Users     UsersIds     `json:"users" bson:"users"`
	Titles    GroupTitleDb `json:"titles" bson:"titles"`
	CreatedAt time.Time    `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt" bson:"updatedAt"`
}

type UsersIds []string

type TitleId string

type GroupTitleDb map[TitleId]GroupTitleItemDb

type GroupTitleItemDb struct {
	TitleId        string            `json:"titleId" bson:"titleId"`
	TitleType      string            `json:"titleType" bson:"titleType"`
	SeasonsWatched *SeasonsWatchedDb `json:"seasonsWatched,omitempty" bson:"seasonsWatched,omitempty"`
	Watched        bool              `json:"watched" bson:"watched"`
	AddedAt        time.Time         `json:"addedAt" bson:"addedAt"`
	UpdatedAt      time.Time         `json:"updatedAt" bson:"updatedAt"`
	WatchedAt      *time.Time        `json:"watchedAt,omitempty" bson:"watchedAt,omitempty"`
}

type SeasonsWatchedDb map[string]SeasonWatchedDb

type SeasonWatchedDb struct {
	Watched   bool       `json:"watched" bson:"watched"`
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
		if mongo.IsDuplicateKeyError(err) {
			return GroupDb{}, ErrDuplicatedRecord
		}
		return GroupDb{}, err
	}

	return group, nil
}

func (db *DB) GroupExists(ctx context.Context, groupId, userId string) (bool, error) {
	coll := db.Collection(GroupsCollection)

	// Only ask MongoDB for the _id field
	opts := options.FindOne().SetProjection(bson.M{"_id": 1})

	err := coll.FindOne(
		ctx,
		bson.M{
			"_id":   groupId,
			"users": bson.M{"$in": []string{userId}}},
		opts).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (db *DB) GroupContainsTitle(ctx context.Context, groupId, titleId, userId string) (bool, error) {
	coll := db.Collection(GroupsCollection)

	// Only ask MongoDB for the _id field
	opts := options.FindOne().SetProjection(bson.M{"_id": 1})

	err := coll.FindOne(
		ctx,
		bson.M{
			"_id":                             groupId,
			"users":                           bson.M{"$in": []string{userId}},
			fmt.Sprintf("titles.%s", titleId): bson.M{"$exists": true},
		},
		opts).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (db *DB) GetGroupById(ctx context.Context, groupId, userId string) (GroupDb, error) {
	coll := db.Collection(GroupsCollection)

	var group GroupDb
	err := coll.FindOne(ctx, bson.M{
		"_id":   groupId,
		"users": bson.M{"$in": []string{userId}},
	}).Decode(&group)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return GroupDb{}, ErrRecordNotFound
		}
		return GroupDb{}, err
	}
	return group, nil
}

func (db *DB) AddUserToGroup(ctx context.Context, groupId, ownerId, userToAddId string) error {
	coll := db.Collection(GroupsCollection)

	result, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": groupId, "users": bson.M{"$in": []string{ownerId}}},
		bson.M{
			"$addToSet": bson.M{"users": userToAddId},
			"$set":      bson.M{"updatedAt": time.Now()},
		},
	)
	if err != nil {
		return err
	}

	// Check if the group was found
	if result.MatchedCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (db *DB) GetUsersFromGroup(ctx context.Context, groupId, userId string) ([]UserDb, error) {
	// First, get the group to find user IDs
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		return []UserDb{}, err
	}

	if len(group.Users) == 0 {
		return []UserDb{}, nil
	}

	// Query users collection with only _id and name fields
	usersColl := db.Collection(UsersCollection)
	filter := bson.M{"_id": bson.M{"$in": group.Users}}

	cursor, err := usersColl.Find(ctx, filter)
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

	now := time.Now()
	newTitle := GroupTitleItemDb{
		TitleId:        titleId,
		TitleType:      "movie",
		SeasonsWatched: nil,
		Watched:        false,
		AddedAt:        now,
		UpdatedAt:      now,
		WatchedAt:      nil,
	}

	_, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": groupId},
		bson.M{
			"$set": bson.M{
				fmt.Sprintf("titles.%s", titleId): newTitle,
				"updatedAt":                       now,
			},
		},
	)
	if err != nil {
		return err
	}
	return nil
}

/*
Update the watched properties of a title in the database.

If the watchedAt is provided but watchedAt.Time is nil,
or watchedAt was set as empty string ("") in request body, watchedAt is set to null in database.
*/
func (db *DB) UpdateGroupTitleWatched(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (*GroupTitleItemDb, error) {
	coll := db.Collection(GroupsCollection)

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	updateDoc := bson.M{}

	if watched != nil {
		updateDoc["watched"] = *watched
	}

	if watchedAt != nil {
		if watchedAt.Time != nil {
			updateDoc["watchedAt"] = *watchedAt.Time
		} else {
			// If watchedAt is provided but Time is nil, set it to null in database
			updateDoc["watchedAt"] = nil
		}
	}

	if len(updateDoc) > 0 {
		now := time.Now()
		updateDoc["updatedAt"] = now
	}

	if len(updateDoc) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Build the update document with direct map key access
	setDoc := bson.M{
		"updatedAt": time.Now(),
	}
	for key, value := range updateDoc {
		setDoc[fmt.Sprintf("titles.%s.%s", titleId, key)] = value
	}

	var updatedGroup GroupDb
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": groupId},
		bson.M{"$set": setDoc},
		opts,
	).Decode(&updatedGroup)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	// Return the updated title from the map using direct key access
	titleKey := TitleId(titleId)
	if title, exists := updatedGroup.Titles[titleKey]; exists {
		return &title, nil
	}

	return nil, ErrRecordNotFound
}

func (db *DB) RemoveTitleFromGroup(ctx context.Context, groupId, titleId, userId string) error {
	coll := db.Collection(GroupsCollection)

	result, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": groupId, "users": bson.M{"$in": []string{userId}}},
		bson.M{
			"$unset": bson.M{
				fmt.Sprintf("titles.%s", titleId): "",
			},
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	if err != nil {
		return err
	}

	// Check if the group was found
	if result.MatchedCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}
