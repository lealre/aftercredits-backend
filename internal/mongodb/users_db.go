package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ----- UserRole enum -----

// UserRole represents the role of a user in the system
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

// ----- Types for the database -----

type UserDb struct {
	Id           string     `json:"id" bson:"_id"`
	Name         string     `json:"name" bson:"name"`
	Email        string     `json:"email" bson:"email"`
	PasswordHash string     `json:"passwordHash" bson:"passwordHash"`
	AvatarURL    *string    `json:"avatarUrl,omitempty" bson:"avatarUrl,omitempty"`
	Groups       []string   `json:"groups,omitempty" bson:"groups,omitempty"`
	Role         UserRole   `json:"role" bson:"role"`
	IsActive     bool       `json:"isActive" bson:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty" bson:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt" bson:"updatedAt"`
}

// ----- Methods for the database -----

func (db *DB) GetUserById(ctx context.Context, id string) (UserDb, error) {
	coll := db.Collection(UsersCollection)
	var userDb UserDb
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&userDb); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return UserDb{}, ErrRecordNotFound
		}
		return UserDb{}, err
	}

	return userDb, nil
}

func (db *DB) GetAllUsers(ctx context.Context) ([]UserDb, error) {
	coll := db.Collection(UsersCollection)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return []UserDb{}, err
	}
	defer cursor.Close(ctx)

	var allUsers []UserDb
	if err := cursor.All(ctx, &allUsers); err != nil {
		return []UserDb{}, err
	}
	return allUsers, nil
}

func (db *DB) UserExists(ctx context.Context, id string) (bool, error) {
	coll := db.Collection(UsersCollection)

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

func (db *DB) AddUser(ctx context.Context, user UserDb) error {
	coll := db.Collection(UsersCollection)
	_, err := coll.InsertOne(ctx, user)
	return err
}
