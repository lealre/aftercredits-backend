package users

import (
	"context"
	"errors"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func getUserByIdDb(ctx context.Context, id string) (User, error) {
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

func getAllUsersDb(ctx context.Context) ([]User, error) {
	coll := mongodb.GetUsersCollection(ctx)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return []User{}, err
	}
	defer cursor.Close(ctx)

	var allUsers []User
	if err := cursor.All(ctx, &allUsers); err != nil {
		return []User{}, err
	}
	return allUsers, nil
}
