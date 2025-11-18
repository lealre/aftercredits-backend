package ratings

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
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

func GetRatingById(db *mongodb.DB, ctx context.Context, ratingId string) (Rating, error) {
	ratingDb, err := db.GetRatingById(ctx, ratingId)
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

func AddRating(db *mongodb.DB, ctx context.Context, rating Rating) error {
	ratingDb := mongodb.RatingDb{
		TitleId: rating.TitleId,
		UserId:  rating.UserId,
		Note:    rating.Note,
	}
	return db.AddRating(ctx, ratingDb)
}

func UpdateRating(db *mongodb.DB, ctx context.Context, ratingId string, updateReq UpdateRatingRequest) error {
	ratingDb := mongodb.RatingDb{
		Id:   ratingId,
		Note: updateReq.Note,
	}
	return db.UpdateRating(ctx, ratingDb)

}
