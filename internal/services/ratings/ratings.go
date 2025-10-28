package ratings

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

// GetRatingsByTitleId retrieves all ratings for a specific title
func GetRatingsByTitleId(ctx context.Context, titleId string) ([]Rating, error) {
	return getRatingsByTitleId(ctx, titleId)
}

// GetRatingById retrieves a single rating by its ID
func GetRatingById(ctx context.Context, ratingId string) (*Rating, error) {
	return getRatingById(ctx, ratingId)
}

func GetRatingsBatch(titleIDs []string, logger *log.Logger) (TitlesRatings, error) {

	filter := bson.M{}
	if len(titleIDs) > 0 {
		filter["titleId"] = bson.M{"$in": titleIDs}
	}

	logger.Printf("Apllying filter for Mongo Search %v", filter)
	ctx := context.Background()
	allRatings, err := getRatingsDb(ctx, filter)
	if err != nil {
		return TitlesRatings{}, err
	}

	logger.Printf("MongoDB returned %v", allRatings)

	titleRatingsMap := TitlesRatings{Titles: map[string][]Rating{}}
	for _, rating := range allRatings {
		if ratingsList, ok := titleRatingsMap.Titles[rating.TitleId]; !ok {
			titleRatingsMap.Titles[rating.TitleId] = []Rating{rating}
		} else {
			titleRatingsMap.Titles[rating.TitleId] = append(ratingsList, rating)
		}
	}
	logger.Printf("Result after map: %v", titleRatingsMap)

	return titleRatingsMap, nil
}
