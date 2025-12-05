package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ----- Types for the database -----

type RatingDb struct {
	Id        string    `json:"id" bson:"_id"`
	TitleId   string    `json:"titleId" bson:"titleId"`
	UserId    string    `json:"userId" bson:"userId"`
	Note      float32   `json:"note" bson:"note"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// ----- Methods for the database -----

func (db *DB) AddRating(ctx context.Context, rating RatingDb) (RatingDb, error) {
	coll := db.Collection(RatingsCollection)

	rating.Id = primitive.NewObjectID().Hex()
	now := time.Now()
	rating.CreatedAt = now
	rating.UpdatedAt = now

	_, err := coll.InsertOne(ctx, rating)
	if err != nil {
		return RatingDb{}, err
	}

	return rating, nil
}

func (db *DB) GetRatingsByTitleId(ctx context.Context, titleId string) ([]RatingDb, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"titleId": titleId}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []RatingDb{}, err
	}
	defer cursor.Close(ctx)

	var ratingsDb []RatingDb
	if err = cursor.All(ctx, &ratingsDb); err != nil {
		return []RatingDb{}, err
	}

	return ratingsDb, nil
}

func (db *DB) GetRatingById(ctx context.Context, ratingId, userId string) (RatingDb, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"_id": ratingId, "userId": userId}

	var rating RatingDb
	err := coll.FindOne(ctx, filter).Decode(&rating)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return RatingDb{}, ErrRecordNotFound
		}
		return RatingDb{}, err
	}

	return rating, nil
}

func (db *DB) GetRatingByUserIdAndTitleId(ctx context.Context, userId, titleId string) (RatingDb, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"userId": userId, "titleId": titleId}

	var rating RatingDb
	err := coll.FindOne(ctx, filter).Decode(&rating)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return RatingDb{}, ErrRecordNotFound
		}
		return RatingDb{}, err
	}

	return rating, nil
}

func (db *DB) DeleteRatingsByTitleId(ctx context.Context, titleId string) (int64, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"titleId": titleId}

	result, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// UpdateRating updates only the Note field of a rating
func (db *DB) UpdateRating(ctx context.Context, ratingDb RatingDb, userId string) error {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"_id": ratingDb.Id, "userId": userId}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"note":      ratingDb.Note,
			"updatedAt": now,
		},
	}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (db *DB) GetRatings(ctx context.Context, args ...any) ([]RatingDb, error) {
	coll := db.Collection(RatingsCollection)

	filter, opts := ResolveFilterAndOptionsSearch(args...)
	cursor, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return []RatingDb{}, err
	}
	defer cursor.Close(ctx)

	var ratingsDb []RatingDb
	if err := cursor.All(ctx, &ratingsDb); err != nil {
		return []RatingDb{}, err
	}

	return ratingsDb, nil
}
