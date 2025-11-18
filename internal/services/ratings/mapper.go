package ratings

import "github.com/lealre/movies-backend/internal/mongodb"

func MapDbRatingDbToApiRating(dbRating mongodb.RatingDb) Rating {
	return Rating(dbRating)
}
