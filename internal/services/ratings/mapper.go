package ratings

import "github.com/lealre/movies-backend/internal/mongodb"

func MapDbRatingDbToApiRating(dbRating mongodb.RatingDb) Rating {
	var seasonsRatings *SeasonsRatings
	if dbRating.SeasonsRatings != nil {
		converted := SeasonsRatings(*dbRating.SeasonsRatings)
		seasonsRatings = &converted
	}

	return Rating{
		Id:             dbRating.Id,
		TitleId:        dbRating.TitleId,
		SeasonsRatings: seasonsRatings,
		UserId:         dbRating.UserId,
		Note:           dbRating.Note,
		CreatedAt:      dbRating.CreatedAt,
		UpdatedAt:      dbRating.UpdatedAt,
	}
}
