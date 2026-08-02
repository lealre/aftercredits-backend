package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ----- Types for the database -----

type RatingDb struct {
	Id             string            `json:"id" bson:"_id"`
	TitleId        string            `json:"titleId" bson:"titleId"`
	SeasonsRatings *SeasonsRatingsDb `json:"seasonsRatings,omitempty" bson:"seasonsRatings,omitempty"`
	UserId         string            `json:"userId" bson:"userId"`
	Note           float32           `json:"note" bson:"note"`
	CreatedAt      time.Time         `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt" bson:"updatedAt"`
}

type SeasonRatingItemDb struct {
	Rating    float32   `json:"rating" bson:"rating"`
	AddedAt   time.Time `json:"addedAt" bson:"addedAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type SeasonsRatingsDb map[string]SeasonRatingItemDb

// ----- Methods for the database -----

func (db *DB) AddRating(ctx context.Context, rating models.UserRating) (models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	ratingDb := userRatingModelToDb(rating)
	ratingDb.Id = primitive.NewObjectID().Hex()
	now := time.Now()
	ratingDb.CreatedAt = now
	ratingDb.UpdatedAt = now

	_, err := coll.InsertOne(ctx, ratingDb)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.UserRating{}, store.ErrDuplicatedRecord
		}
		return models.UserRating{}, err
	}

	return userRatingDbToModel(ratingDb), nil
}

func (db *DB) GetRatingsByTitleId(ctx context.Context, titleId string) ([]models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"titleId": titleId}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []models.UserRating{}, err
	}
	defer cursor.Close(ctx)

	var ratingsDb []RatingDb
	if err = cursor.All(ctx, &ratingsDb); err != nil {
		return []models.UserRating{}, err
	}

	ratings := make([]models.UserRating, len(ratingsDb))
	for i, r := range ratingsDb {
		ratings[i] = userRatingDbToModel(r)
	}

	return ratings, nil
}

func (db *DB) GetRatingById(ctx context.Context, ratingId, userId string) (models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"_id": ratingId, "userId": userId}

	var ratingDb RatingDb
	err := coll.FindOne(ctx, filter).Decode(&ratingDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.UserRating{}, store.ErrRecordNotFound
		}
		return models.UserRating{}, err
	}

	return userRatingDbToModel(ratingDb), nil
}

func (db *DB) GetRatingByUserIdAndTitleId(ctx context.Context, userId, titleId string) (models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"userId": userId, "titleId": titleId}

	var ratingDb RatingDb
	err := coll.FindOne(ctx, filter).Decode(&ratingDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.UserRating{}, store.ErrRecordNotFound
		}
		return models.UserRating{}, err
	}

	return userRatingDbToModel(ratingDb), nil
}

func (db *DB) UpdateRating(ctx context.Context, rating models.UserRating, userId string) (models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"_id": rating.Id, "userId": userId}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"note":           rating.Note,
			"seasonsRatings": seasonsRatingsModelToDb(rating.SeasonsRatings),
			"updatedAt":      now,
		},
	}

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	var updatedRatingDb RatingDb
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedRatingDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.UserRating{}, store.ErrRecordNotFound
		}
		return models.UserRating{}, err
	}

	return userRatingDbToModel(updatedRatingDb), nil
}

// GetRatingsByTitleIds fetches all ratings whose titleId is in titleIds. When
// titleIds is empty, it returns all ratings (mirrors the previous
// GetRatings(...any) behavior when called with an empty filter).
func (db *DB) GetRatingsByTitleIds(ctx context.Context, titleIds []string) ([]models.UserRating, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{}
	if len(titleIds) > 0 {
		filter["titleId"] = bson.M{"$in": titleIds}
	}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []models.UserRating{}, err
	}
	defer cursor.Close(ctx)

	var ratingsDb []RatingDb
	if err := cursor.All(ctx, &ratingsDb); err != nil {
		return []models.UserRating{}, err
	}

	ratings := make([]models.UserRating, len(ratingsDb))
	for i, r := range ratingsDb {
		ratings[i] = userRatingDbToModel(r)
	}

	return ratings, nil
}

func (db *DB) DeleteRating(ctx context.Context, ratingId, userId string) (int64, error) {
	coll := db.Collection(RatingsCollection)

	filter := bson.M{"_id": ratingId, "userId": userId}

	result, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, store.ErrRecordNotFound
		}
		return 0, err
	}

	if result.DeletedCount == 0 {
		return 0, store.ErrRecordNotFound
	}

	return result.DeletedCount, nil
}
