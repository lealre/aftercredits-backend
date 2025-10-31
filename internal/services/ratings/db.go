package ratings

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddRating(ctx context.Context, rating Rating) error {
	coll := mongodb.GetRatingsCollection(ctx)

	rating.Id = primitive.NewObjectID().Hex()

	doc := map[string]any{
		"_id":      rating.Id,
		"titleId":  rating.TitleId,
		"userId":   rating.UserId,
		"note":     rating.Note,
		"comments": rating.Comments,
	}

	_, err := coll.InsertOne(ctx, doc)
	return err
}

func getRatingsByTitleId(ctx context.Context, titleId string) ([]Rating, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	filter := bson.M{"titleId": titleId}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ratings []Rating
	if err = cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}

	return ratings, nil
}

func getRatingById(ctx context.Context, ratingId string) (*Rating, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	filter := bson.M{"_id": ratingId}

	var rating Rating
	err := coll.FindOne(ctx, filter).Decode(&rating)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, mongodb.ErrRecordNotFound
		}
		return nil, err
	}

	return &rating, nil
}

func DeleteRatingsByTitleId(ctx context.Context, titleId string) (int64, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	filter := bson.M{"titleId": titleId}

	result, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// UpdateRating updates only the Note and Comments fields of a rating
func UpdateRating(ctx context.Context, ratingId string, updateReq UpdateRatingRequest) error {
	coll := mongodb.GetRatingsCollection(ctx)

	filter := bson.M{"_id": ratingId}

	update := bson.M{
		"$set": bson.M{
			"note":     updateReq.Note,
			"comments": updateReq.Comments,
		},
	}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return mongodb.ErrRecordNotFound
	}

	return nil
}

func getRatingsDb(ctx context.Context, args ...any) ([]Rating, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	filter, opts := mongodb.ResolveFilterAndOptionsSearch(args...)
	cursor, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ratings []Rating
	if err := cursor.All(ctx, &ratings); err != nil {
		return nil, err
	}

	return ratings, nil
}
