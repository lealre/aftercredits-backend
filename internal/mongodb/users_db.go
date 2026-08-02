package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
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

func (db *DB) GetUserById(ctx context.Context, id string) (models.User, error) {
	coll := db.Collection(UsersCollection)
	var userDb UserDb
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&userDb); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.User{}, store.ErrRecordNotFound
		}
		return models.User{}, err
	}

	return userDbToModel(userDb), nil
}

func (db *DB) GetUserByUsernameOrEmail(ctx context.Context, username, email string) (models.User, error) {
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
			return models.User{}, store.ErrRecordNotFound
		}
		return models.User{}, err
	}

	return userDbToModel(userDb), nil
}

func (db *DB) GetAllUsers(ctx context.Context) ([]models.User, error) {
	coll := db.Collection(UsersCollection)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return []models.User{}, err
	}
	defer cursor.Close(ctx)

	var allUsersDb []UserDb
	if err := cursor.All(ctx, &allUsersDb); err != nil {
		return []models.User{}, err
	}

	allUsers := make([]models.User, 0, len(allUsersDb))
	for _, userDb := range allUsersDb {
		allUsers = append(allUsers, userDbToModel(userDb))
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

func (db *DB) AddUser(ctx context.Context, user models.User) error {
	coll := db.Collection(UsersCollection)
	_, err := coll.InsertOne(ctx, userModelToDb(user))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	return nil
}

func (db *DB) DeleteUserById(ctx context.Context, id string) error {
	coll := db.Collection(UsersCollection)
	_, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return store.ErrRecordNotFound
		}
		return err
	}
	return err
}

func (db *DB) UpdateUserInfo(ctx context.Context, id string, user models.User) (models.User, error) {
	coll := db.Collection(UsersCollection)

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	now := time.Now()
	userDb := userModelToDb(user)
	userDb.UpdatedAt = now

	var updatedUserDb UserDb
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": userDb},
		opts,
	).Decode(&updatedUserDb)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.User{}, store.ErrDuplicatedRecord
		}
		return models.User{}, err
	}

	return userDbToModel(updatedUserDb), nil
}

func (db *DB) UpdateUserLastLoginAt(ctx context.Context, userId string) (models.User, error) {
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
			return models.User{}, store.ErrRecordNotFound
		}
		return models.User{}, err
	}

	return userDbToModel(updatedUser), nil
}

func (db *DB) UpdateUserGroup(ctx context.Context, userId string, groupId string) (models.User, error) {
	coll := db.Collection(UsersCollection)

	now := time.Now()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedUser UserDb
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": userId},
		bson.M{"$addToSet": bson.M{"groups": groupId}, "$set": bson.M{"updatedAt": now}},
		opts,
	).Decode(&updatedUser)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.User{}, store.ErrRecordNotFound
		}
		return models.User{}, err
	}

	return userDbToModel(updatedUser), nil
}

// RemoveGroupFromUser pulls a group id from a user's groups array.
func (db *DB) RemoveGroupFromUser(ctx context.Context, userId, groupId string) error {
	coll := db.Collection(UsersCollection)
	_, err := coll.UpdateOne(ctx,
		bson.M{"_id": userId},
		bson.M{"$pull": bson.M{"groups": groupId}, "$set": bson.M{"updatedAt": time.Now()}},
	)
	return err
}
