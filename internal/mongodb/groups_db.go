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

/*
Update the watched properties of a title in the database.

If the watchedAt is provided but watchedAt.Time is nil,
or watchedAt was set as empty string ("") in request body, watchedAt is set to null in database.

TODO: If watchedAt is provided but whatched is false or nil, do not proceed with the update.
*/
func (db *DB) UpdateGroupTitleWatched(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (*GroupTitleDb, error) {
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

	// Build the update document with array filters to update the nested array element
	setDoc := bson.M{}
	for key, value := range updateDoc {
		setDoc["titles.$[elem]."+key] = value
	}

	// Use array filters to match the specific title by titleId
	// Note: The BSON field name is "titleId" (from the struct tag), not "id"
	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"elem.titleId": titleId},
		},
	}
	opts.SetArrayFilters(arrayFilters)

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

	// Find and return the updated title from the group
	for _, title := range updatedGroup.Titles {
		if title.Id == titleId {
			return &title, nil
		}
	}

	return nil, ErrRecordNotFound
}

func (db *DB) RemoveTitleFromGroup(ctx context.Context, groupId string, titleId string) error {
	coll := db.Collection(GroupsCollection)

	result, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": groupId},
		bson.M{"$pull": bson.M{"titles": bson.M{"titleId": titleId}}},
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
