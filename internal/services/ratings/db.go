package ratings

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AddRating inserts a rating document into the ratings collection
func AddRating(ctx context.Context, rating Rating) error {
	coll := mongodb.GetRatingsCollection(ctx)

	// Generate a new ObjectID for the rating
	rating.Id = primitive.NewObjectID().Hex()

	// Convert to map for insertion
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

// getRatingsByTitleId retrieves all ratings for a specific title
func getRatingsByTitleId(ctx context.Context, titleId string) ([]Rating, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	// Create filter to find ratings by titleId
	filter := bson.M{"titleId": titleId}

	// Find all ratings matching the titleId
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

// getRatingById retrieves a single rating by its ID
func getRatingById(ctx context.Context, ratingId string) (*Rating, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	// Create filter to find rating by ID
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

// DeleteRatingsByTitleId deletes all ratings for a specific title
func DeleteRatingsByTitleId(ctx context.Context, titleId string) (int64, error) {
	coll := mongodb.GetRatingsCollection(ctx)

	// Create filter to find ratings by titleId
	filter := bson.M{"titleId": titleId}

	// Delete all ratings matching the titleId
	result, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// UpdateRating updates only the Note and Comments fields of a rating
func UpdateRating(ctx context.Context, ratingId string, updateReq UpdateRatingRequest) error {
	coll := mongodb.GetRatingsCollection(ctx)

	// Create filter to find rating by ID
	filter := bson.M{"_id": ratingId}

	// Create update document with only Note and Comments
	update := bson.M{
		"$set": bson.M{
			"note":     updateReq.Note,
			"comments": updateReq.Comments,
		},
	}

	// Perform the update
	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// Check if any document was modified
	if result.MatchedCount == 0 {
		return mongodb.ErrRecordNotFound
	}

	return nil
}
