package ratings

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetRatingsByTitleId(db *mongodb.DB, ctx context.Context, titleId string) ([]Rating, error) {
	ratingsDb, err := db.GetRatingsByTitleId(ctx, titleId)
	if err != nil {
		return []Rating{}, err
	}

	var ratings []Rating
	for _, ratingDb := range ratingsDb {
		ratings = append(ratings, MapDbRatingDbToApiRating(ratingDb))
	}

	return ratings, nil
}

func GetRatingById(db *mongodb.DB, ctx context.Context, ratingId, userId string) (Rating, error) {
	ratingDb, err := db.GetRatingById(ctx, ratingId, userId)
	if err != nil {
		return Rating{}, err
	}

	return MapDbRatingDbToApiRating(ratingDb), nil
}

func GetRatingsBatch(db *mongodb.DB, ctx context.Context, titleIDs []string) (TitlesRatings, error) {

	filter := bson.M{}
	if len(titleIDs) > 0 {
		filter["titleId"] = bson.M{"$in": titleIDs}
	}

	allRatingsDb, err := db.GetRatings(ctx, filter)
	if err != nil {
		return TitlesRatings{}, err
	}

	titleRatingsMap := TitlesRatings{Titles: map[string][]Rating{}}
	for _, ratingDb := range allRatingsDb {
		rating := MapDbRatingDbToApiRating(ratingDb)
		if ratingsList, ok := titleRatingsMap.Titles[rating.TitleId]; !ok {
			titleRatingsMap.Titles[rating.TitleId] = []Rating{rating}
		} else {
			titleRatingsMap.Titles[rating.TitleId] = append(ratingsList, rating)
		}
	}

	return titleRatingsMap, nil
}

func AddRating(db *mongodb.DB, ctx context.Context, rating NewRating, userId string) (Rating, error) {
	// Check if rating already exists
	_, err := db.GetRatingByUserIdAndTitleId(ctx, userId, rating.TitleId)
	if err == nil {
		// Rating already exists
		return Rating{}, ErrRatingAlreadyExists
	}
	if err != mongodb.ErrRecordNotFound {
		// Some other error occurred
		return Rating{}, err
	}

	ratingDb := mongodb.RatingDb{
		TitleId: rating.TitleId,
		UserId:  userId,
		Note:    rating.Note,
	}

	ratingDb, err = db.AddRating(ctx, ratingDb)
	if err != nil {
		// Fallback: check for duplicate key error in case the check above missed it
		if mongo.IsDuplicateKeyError(err) {
			return Rating{}, ErrRatingAlreadyExists
		}
		return Rating{}, err
	}

	return MapDbRatingDbToApiRating(ratingDb), nil
}

func UpdateRating(db *mongodb.DB, ctx context.Context, ratingId, userId string, updateReq UpdateRatingRequest) error {
	ratingDb := mongodb.RatingDb{
		Id:   ratingId,
		Note: updateReq.Note,
	}
	return db.UpdateRating(ctx, ratingDb, userId)
}
