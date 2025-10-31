package ratings

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func GetRatingsByTitleId(ctx context.Context, titleId string) ([]Rating, error) {
	return getRatingsByTitleId(ctx, titleId)
}

func GetRatingById(ctx context.Context, ratingId string) (*Rating, error) {
	return getRatingById(ctx, ratingId)
}

func GetRatingsBatch(titleIDs []string) (TitlesRatings, error) {

	filter := bson.M{}
	if len(titleIDs) > 0 {
		filter["titleId"] = bson.M{"$in": titleIDs}
	}

	ctx := context.Background()
	allRatings, err := getRatingsDb(ctx, filter)
	if err != nil {
		return TitlesRatings{}, err
	}

	titleRatingsMap := TitlesRatings{Titles: map[string][]Rating{}}
	for _, rating := range allRatings {
		if ratingsList, ok := titleRatingsMap.Titles[rating.TitleId]; !ok {
			titleRatingsMap.Titles[rating.TitleId] = []Rating{rating}
		} else {
			titleRatingsMap.Titles[rating.TitleId] = append(ratingsList, rating)
		}
	}

	return titleRatingsMap, nil
}
