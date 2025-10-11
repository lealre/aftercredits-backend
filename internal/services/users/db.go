package users

import (
	"context"
	"errors"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func getUserByID(ctx context.Context, id string) (User, error) {
	coll := mongodb.GetUsersCollection(ctx)
	var user User
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return User{}, mongodb.ErrRecordNotFound
		}
		return User{}, err
	}

	return user, nil
}

// GetAllUsers fetches all user documents from the collection
func GetAllUsers(ctx context.Context) (*mongo.Cursor, error) {
	coll := mongodb.GetUsersCollection(ctx)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return cursor, nil
}
