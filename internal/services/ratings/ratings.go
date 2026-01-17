package ratings

import (
	"context"
	"errors"
	"fmt"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
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
		// This aggregates users ratings for the same title
		if ratingsList, ok := titleRatingsMap.Titles[rating.TitleId]; !ok {
			titleRatingsMap.Titles[rating.TitleId] = []Rating{rating}
		} else {
			titleRatingsMap.Titles[rating.TitleId] = append(ratingsList, rating)
		}
	}

	return titleRatingsMap, nil
}

// The behavior differs based on whether the title is a TV series or not.
//
// 1 - For TV Series (tvSeries or tvMiniSeries):
//
//	1.1. Validates that a season number is provided in the rating request
//	1.2. Validates that the season exists in the title's seasons list
//	1.3. Checks if a rating already exists for this user/title combination:
//	   - If no rating exists:
//			1.3.1. Creates a new rating with the season rating
//	   - If a rating exists:
//			1.3.2. Checks if a rating for this specific season already exists
//			1.3.3. If the season rating exists: Returns ErrSeasonRatingAlreadyExists
//			1.3.4. If the season rating doesn't exist: Adds the new season rating to the existing rating
//	1.4. Calculates the overall rating as the mean of all season ratings
//
// 2 - For Non-TV Series (movies, etc.):
//
//	2.1. Checks if a rating already exists for this user/title combination
//	2.2. If a rating exists: Returns ErrRatingAlreadyExists (only one rating per user/title allowed)
//
// 3 - Creates the rating in the database
//
// Parameters:
//   - db: MongoDB database connection
//   - ctx: Context for the operation
//   - rating: NewRating struct containing titleId, note, and optional season number
//   - userId: ID of the user creating the rating
//
// Returns:
//   - Rating: The created rating with all fields populated
//   - error: Returns various errors based on validation failures:
//   - ErrInvalidNoteValue: If note is not between 0 and 10
//   - ErrSeasonRequired: If season is missing for TV series
//   - ErrSeasonDoesNotExist: If the season doesn't exist in the title
//   - ErrSeasonRatingAlreadyExists: If rating for this season already exists (TV series only)
//   - ErrRatingAlreadyExists: If rating already exists (non-TV series)
func AddRating(db *mongodb.DB, ctx context.Context, rating NewRating, userId string) (Rating, error) {
	if rating.Note < 0 || rating.Note > 10 {
		return Rating{}, ErrInvalidNoteValue
	}
	if rating.Season != nil && *rating.Season < 0 {
		return Rating{}, ErrInvalidSeasonValue
	}

	title, err := titles.GetTitleById(db, ctx, rating.TitleId)
	if err != nil {
		return Rating{}, err
	}

	var seasonsRatings *mongodb.SeasonsRatingsDb
	newRating := rating.Note

	// 1.3 / 2.1: Get existing rating (used for both TV and non-TV series)
	existingRating, err := db.GetRatingByUserIdAndTitleId(ctx, userId, rating.TitleId)
	hasExistingRating := err == nil
	if err != nil && err != mongodb.ErrRecordNotFound {
		return Rating{}, err
	}

	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		// 1.1: Validates that a season number is provided
		if rating.Season == nil {
			return Rating{}, ErrSeasonRequired
		}

		// 1.2: Validates that the season exists in the title's seasons list
		seasonExists := false
		for _, season := range title.Seasons {
			if season.Season == fmt.Sprintf("%d", *rating.Season) {
				seasonExists = true
				break
			}
		}
		if !seasonExists {
			return Rating{}, ErrSeasonDoesNotExist
		}

		// 1.3: Checks if a rating already exists for this user/title combination
		if !hasExistingRating {
			// 1.3.1: Creates a new rating with the season rating
			seasonsRatings = &mongodb.SeasonsRatingsDb{
				*rating.Season: rating.Note,
			}
		} else {
			// 1.3.2: Checks if a rating for this specific season already exists
			if existingRating.SeasonsRatings != nil {
				if _, exists := (*existingRating.SeasonsRatings)[*rating.Season]; exists {
					// 1.3.3: Returns ErrSeasonRatingAlreadyExists
					return Rating{}, ErrSeasonRatingAlreadyExists
				}
			}
			// 1.3.4: Adds the new season rating to the existing rating
			if existingRating.SeasonsRatings == nil {
				seasonsRatings = &mongodb.SeasonsRatingsDb{
					*rating.Season: rating.Note,
				}
			} else {
				seasonsRatings = existingRating.SeasonsRatings
				(*seasonsRatings)[*rating.Season] = rating.Note
			}
		}

		// 1.4: Calculates the overall rating as the mean of all season ratings
		var sum float32
		var count int
		for _, seasonRating := range *seasonsRatings {
			sum += seasonRating
			count++
		}
		newRating = sum / float32(count)
	} else {
		// 2.1: Checks if a rating already exists for this user/title combination
		// 2.2: If a rating exists, returns ErrRatingAlreadyExists
		if hasExistingRating {
			return Rating{}, ErrRatingAlreadyExists
		}
	}

	// 3: Creates the rating in the database
	ratingDb := mongodb.RatingDb{
		TitleId:        rating.TitleId,
		UserId:         userId,
		Note:           newRating,
		SeasonsRatings: seasonsRatings,
	}

	ratingDb, err = db.AddRating(ctx, ratingDb)
	if err != nil {
		if errors.Is(err, mongodb.ErrDuplicatedRecord) {
			return Rating{}, ErrRatingAlreadyExists
		}
		return Rating{}, err
	}

	return MapDbRatingDbToApiRating(ratingDb), nil
}

func UpdateRating(db *mongodb.DB, ctx context.Context, ratingId, userId string, updateReq UpdateRatingRequest) (Rating, error) {

	if updateReq.Note < 0 || updateReq.Note > 10 {
		return Rating{}, ErrInvalidNoteValue
	}

	ratingDb := mongodb.RatingDb{
		Id:   ratingId,
		Note: updateReq.Note,
	}

	updatedRatingDb, err := db.UpdateRating(ctx, ratingDb, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return Rating{}, ErrRatingNotFound
		}
		return Rating{}, err
	}
	return MapDbRatingDbToApiRating(updatedRatingDb), nil
}
