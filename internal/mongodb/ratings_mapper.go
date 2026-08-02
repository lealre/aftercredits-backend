package mongodb

import "github.com/lealre/movies-backend/internal/models"

// userRatingDbToModel converts the mongo-specific RatingDb into the
// storage-neutral models.UserRating used by the service layer.
func userRatingDbToModel(r RatingDb) models.UserRating {
	return models.UserRating{
		Id:             r.Id,
		TitleId:        r.TitleId,
		SeasonsRatings: seasonsRatingsDbToModel(r.SeasonsRatings),
		UserId:         r.UserId,
		Note:           r.Note,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// userRatingModelToDb converts a storage-neutral models.UserRating back into
// the mongo-specific RatingDb used at the persistence boundary.
func userRatingModelToDb(r models.UserRating) RatingDb {
	return RatingDb{
		Id:             r.Id,
		TitleId:        r.TitleId,
		SeasonsRatings: seasonsRatingsModelToDb(r.SeasonsRatings),
		UserId:         r.UserId,
		Note:           r.Note,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// ----- SeasonsRatings -----

func seasonRatingItemDbToModel(s SeasonRatingItemDb) models.SeasonRatingItem {
	return models.SeasonRatingItem{
		Rating:    s.Rating,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonRatingItemModelToDb(s models.SeasonRatingItem) SeasonRatingItemDb {
	return SeasonRatingItemDb{
		Rating:    s.Rating,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonsRatingsDbToModel(s *SeasonsRatingsDb) *models.SeasonsRatings {
	if s == nil {
		return nil
	}
	out := make(models.SeasonsRatings, len(*s))
	for k, v := range *s {
		out[k] = seasonRatingItemDbToModel(v)
	}
	return &out
}

func seasonsRatingsModelToDb(s *models.SeasonsRatings) *SeasonsRatingsDb {
	if s == nil {
		return nil
	}
	out := make(SeasonsRatingsDb, len(*s))
	for k, v := range *s {
		out[k] = seasonRatingItemModelToDb(v)
	}
	return &out
}
