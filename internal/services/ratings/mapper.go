package ratings

import "github.com/lealre/movies-backend/internal/models"

func MapDbRatingDbToApiRating(dbRating models.UserRating) Rating {
	var seasonsRatings *SeasonsRatings
	if dbRating.SeasonsRatings != nil {
		converted := make(SeasonsRatings)
		for seasonKey, seasonRatingItem := range *dbRating.SeasonsRatings {
			converted[seasonKey] = SeasonRating{
				Rating:    seasonRatingItem.Rating,
				AddedAt:   seasonRatingItem.AddedAt,
				UpdatedAt: seasonRatingItem.UpdatedAt,
			}
		}
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
