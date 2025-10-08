package ratings

import "context"

// GetRatingsByTitleId retrieves all ratings for a specific title
func GetRatingsByTitleId(ctx context.Context, titleId string) ([]Rating, error) {
	return getRatingsByTitleId(ctx, titleId)
}

// GetRatingById retrieves a single rating by its ID
func GetRatingById(ctx context.Context, ratingId string) (*Rating, error) {
	return getRatingById(ctx, ratingId)
}
