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
	Email        string     `json:"email,omitempty" bson:"email,omitempty"`
	Username     string     `json:"username" bson:"username,omitempty"`
	PasswordHash string     `json:"passwordHash,omitempty" bson:"passwordHash,omitempty"`
	AvatarURL    *string    `json:"avatarUrl,omitempty" bson:"avatarUrl,omitempty"`
	Groups       []string   `json:"groups,omitempty" bson:"groups,omitempty"`
	Role         UserRole   `json:"role,omitempty" bson:"role,omitempty"`
	IsActive     bool       `json:"isActive,omitempty" bson:"isActive,omitempty"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty" bson:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
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

func (db *DB) GetUserByUsernameOrEmail(ctx context.Context, username, email string) (UserDb, error) {
	coll := db.Collection(UsersCollection)

	filter := bson.M{}
	if username != "" {
		filter["username"] = username
	}

	if email != "" {
		filter["email"] = email
	}

	var userDb UserDb
	if err := coll.FindOne(ctx, filter).Decode(&userDb); err != nil {
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

func (db *DB) DeleteUserById(ctx context.Context, id string) error {
	coll := db.Collection(UsersCollection)
	_, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrRecordNotFound
		}
		return err
	}
	return err
}

func (db *DB) UpdateUserInfo(ctx context.Context, id string, user UserDb) (UserDb, error) {
	coll := db.Collection(UsersCollection)

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	now := time.Now()
	user.UpdatedAt = now

	var updatedUserDb UserDb
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": user},
		opts,
	).Decode(&updatedUserDb)
	if err != nil {
		return UserDb{}, err
	}

	return updatedUserDb, nil
}

func (db *DB) UpdateUserLastLoginAt(ctx context.Context, userId string) (UserDb, error) {
	coll := db.Collection(UsersCollection)

	now := time.Now()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedUser UserDb
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": userId},
		bson.M{"$set": bson.M{"lastLoginAt": now}},
		opts,
	).Decode(&updatedUser)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return UserDb{}, ErrRecordNotFound
		}
		return UserDb{}, err
	}

	return updatedUser, nil
}
