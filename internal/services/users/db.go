package users

import (
	"context"
	"errors"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func getUserByID(ctx context.Context, id string) (bson.M, error) {
	coll := mongodb.GetUsersCollection(ctx)
	var out bson.M
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&out); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongodb.ErrRecordNotFound
		}
		return nil, err
	}

	return out, nil
}
