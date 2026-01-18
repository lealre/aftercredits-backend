package ratings

import (
	"context"
	"errors"
	"strconv"

	"github.com/lealre/movies-backend/internal/logx"
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

// AddRating creates a new rating for a title.
//
// Routes to the appropriate handler based on title type (TV series or movie):
//   - addRatingForTVSeries: If the title is a TV series (tvSeries or tvMiniSeries)
//   - addRatingForMovie: If the title is a movie (non-TV series)
//
// Returns:
//   - Rating: The created or updated rating with all fields populated
//   - error: Returns various errors based on validation failures from routes handlers
func AddRating(db *mongodb.DB, ctx context.Context, rating NewRating, userId string) (Rating, error) {
	logger := logx.FromContext(ctx)

	if rating.Note < 0 || rating.Note > 10 {
		return Rating{}, ErrInvalidNoteValue
	}
	if rating.Season != nil && *rating.Season <= 0 {
		return Rating{}, ErrInvalidSeasonValue
	}

	title, err := titles.GetTitleById(db, ctx, rating.TitleId)
	if err != nil {
		return Rating{}, err
	}

	// Split logic for TV series and non-TV series
	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Adding rating for TV series %s", rating.TitleId)
		return addRatingForTVSeries(db, ctx, rating, userId, title)
	} else {
		logger.Printf("Adding rating for movie %s", rating.TitleId)
		return addRatingForMovie(db, ctx, rating, userId)
	}
}

// addRatingForTVSeries handles rating creation/update for TV series (tvSeries or tvMiniSeries).
//
//	1.1. Validates that a season number is provided in the rating request
//	1.2. Validates that the season exists in the title's seasons list
//	1.3. Checks if a rating already exists for this user/title combination:
//	   - If no rating exists:
//	     1.3.1. Creates a new rating with the season rating
//	   - If a rating exists:
//	     1.3.2. Checks if a rating for this specific season already exists
//	     1.3.3. If the season rating exists: Returns ErrSeasonRatingAlreadyExists
//	     1.3.4. If the season rating doesn't exist: Adds the new season rating to the existing rating
//	1.4. Calculates the overall rating as the mean of all season ratings
//	1.5. Creates a new rating OR updates the existing rating in the database
//
// Parameters:
//   - db: MongoDB database connection
//   - ctx: Context for the operation
//   - rating: NewRating struct containing titleId, note, and season number
//   - userId: ID of the user creating the rating
//   - title: Title struct with seasons information
//
// Returns:
//   - Rating: The created or updated rating with all fields populated
//   - error: Returns various errors based on validation failures:
//   - ErrSeasonRequired: If season is missing
//   - ErrSeasonDoesNotExist: If the season doesn't exist in the title
//   - ErrSeasonRatingAlreadyExists: If rating for this season already exists
func addRatingForTVSeries(db *mongodb.DB, ctx context.Context, newRating NewRating, userId string, title titles.Title) (Rating, error) {
	// 1.1: Validates that a season number is provided
	if newRating.Season == nil {
		return Rating{}, ErrSeasonRequired
	}

	newSeasonAsString := strconv.Itoa(*newRating.Season)

	// 1.2: Validates that the season exists in the title's seasons list
	seasonExists := false
	for _, season := range title.Seasons {
		if season.Season == newSeasonAsString {
			seasonExists = true
			break
		}
	}
	if !seasonExists {
		return Rating{}, ErrSeasonDoesNotExist
	}

	// 1.3: Checks if a rating already exists for this user/title combination
	existingRating, err := db.GetRatingByUserIdAndTitleId(ctx, userId, newRating.TitleId)
	hasExistingRating := err == nil
	if err != nil && err != mongodb.ErrRecordNotFound {
		return Rating{}, err
	}

	var seasonsRatings *mongodb.SeasonsRatingsDb

	if !hasExistingRating {
		// 1.3.1: Creates a new rating with the season rating
		seasonsRatings = &mongodb.SeasonsRatingsDb{
			newSeasonAsString: newRating.Note,
		}
	} else {
		// 1.3.2: Checks if a rating for this specific season already exists
		if existingRating.SeasonsRatings != nil {
			if _, exists := (*existingRating.SeasonsRatings)[newSeasonAsString]; exists {
				// 1.3.3: Returns ErrSeasonRatingAlreadyExists
				return Rating{}, ErrSeasonRatingAlreadyExists
			}
		}
		// 1.3.4: Adds the new season rating to the existing rating
		if existingRating.SeasonsRatings == nil {
			seasonsRatings = &mongodb.SeasonsRatingsDb{
				newSeasonAsString: newRating.Note,
			}
		} else {
			seasonsRatings = existingRating.SeasonsRatings
			(*seasonsRatings)[newSeasonAsString] = newRating.Note
		}
	}

	// 1.4: Calculates the overall rating as the mean of all season ratings
	var sum float32
	var count int
	for _, seasonRating := range *seasonsRatings {
		sum += seasonRating
		count++
	}
	newOverallRating := sum / float32(count)

	// 1.5: Creates a new rating OR updates the existing rating in the database
	ratingDb := mongodb.RatingDb{
		TitleId:        newRating.TitleId,
		UserId:         userId,
		Note:           newOverallRating,
		SeasonsRatings: seasonsRatings,
	}

	if hasExistingRating {
		// Update existing rating
		ratingDb.Id = existingRating.Id
		ratingDb.CreatedAt = existingRating.CreatedAt
		updatedRatingDb, err := db.UpdateRating(ctx, ratingDb, userId)
		if err != nil {
			return Rating{}, err
		}
		return MapDbRatingDbToApiRating(updatedRatingDb), nil
	} else {
		// Create new rating
		ratingDb, err = db.AddRating(ctx, ratingDb)
		if err != nil {
			return Rating{}, err
		}
		return MapDbRatingDbToApiRating(ratingDb), nil
	}
}

// addRatingForMovie handles rating creation for movies (non-TV series).
//
//	1.1. Checks if a rating already exists for this user/title combination
//	1.2. If a rating exists: Returns ErrRatingAlreadyExists (only one rating per user/title allowed)
//	1.3. If no rating exists: Creates a new rating with the provided note value
//
// Parameters:
//   - db: MongoDB database connection
//   - ctx: Context for the operation
//   - rating: NewRating struct containing titleId and note
//   - userId: ID of the user creating the rating
//
// Returns:
//   - Rating: The created rating with all fields populated
//   - error: Returns various errors based on validation failures:
//   - ErrRatingAlreadyExists: If rating already exists
func addRatingForMovie(db *mongodb.DB, ctx context.Context, rating NewRating, userId string) (Rating, error) {
	// 1.1: Checks if a rating already exists for this user/title combination
	_, err := db.GetRatingByUserIdAndTitleId(ctx, userId, rating.TitleId)
	if err == nil {
		// 1.2: If a rating exists, returns ErrRatingAlreadyExists
		return Rating{}, ErrRatingAlreadyExists
	} else if err != mongodb.ErrRecordNotFound {
		return Rating{}, err
	}

	// 1.3: If no rating exists, creates a new rating with the provided note value
	ratingDb := mongodb.RatingDb{
		TitleId: rating.TitleId,
		UserId:  userId,
		Note:    rating.Note,
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
	logger := logx.FromContext(ctx)

	if updateReq.Note < 0 || updateReq.Note > 10 {
		return Rating{}, ErrInvalidNoteValue
	}
	if updateReq.Season != nil && *updateReq.Season <= 0 {
		return Rating{}, ErrInvalidSeasonValue
	}

	rating, err := GetRatingById(db, ctx, ratingId, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return Rating{}, ErrRatingNotFound
		}
		return Rating{}, err
	}

	title, err := titles.GetTitleById(db, ctx, rating.TitleId)
	if err != nil {
		return Rating{}, err
	}

	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Updating rating for TV series %s", rating.TitleId)
		return updateRatingForTVSeries(db, ctx, rating, userId, updateReq)
	} else {
		logger.Printf("Updating rating for movie %s", rating.TitleId)
		return updateRatingForMovie(db, ctx, ratingId, userId, updateReq)
	}
}

func updateRatingForMovie(db *mongodb.DB, ctx context.Context, ratingId, userId string, updateReq UpdateRatingRequest) (Rating, error) {
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

func updateRatingForTVSeries(db *mongodb.DB, ctx context.Context, rating Rating, userId string, updateReq UpdateRatingRequest) (Rating, error) {
	if updateReq.Season == nil {
		return Rating{}, ErrSeasonRequired
	}

	// This is a sanity check, if it fails here, something is wrong in rating creation
	if rating.SeasonsRatings == nil {
		return Rating{}, ErrRatingNotFound
	}

	// check if the season exists in the rating
	newSeasonAsString := strconv.Itoa(*updateReq.Season)
	if _, exists := (*rating.SeasonsRatings)[newSeasonAsString]; !exists {
		return Rating{}, ErrSeasonDoesNotExist
	}

	// update the season rating
	(*rating.SeasonsRatings)[newSeasonAsString] = updateReq.Note

	// calculate the overall rating
	var sum float32
	var count int
	for _, seasonRating := range *rating.SeasonsRatings {
		sum += seasonRating
		count++
	}
	newOverallRating := sum / float32(count)

	// update the overall rating
	rating.Note = newOverallRating
	var seasonsRatings *mongodb.SeasonsRatingsDb
	converted := mongodb.SeasonsRatingsDb(*rating.SeasonsRatings)
	seasonsRatings = &converted

	ratingDb := mongodb.RatingDb{
		Id:             rating.Id,
		Note:           newOverallRating,
		SeasonsRatings: seasonsRatings,
	}

	// update the rating in the database
	updatedRatingDb, err := db.UpdateRating(ctx, ratingDb, userId)
	if err != nil {
		return Rating{}, err
	}

	return MapDbRatingDbToApiRating(updatedRatingDb), nil
}
