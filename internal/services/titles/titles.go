package titles

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetPageOfTitles(
	db *mongodb.DB,
	ctx context.Context,
	size, page int,
	orderByField string,
	ascending *bool,
	titleIds []string,
) (generics.Page[Title], error) {

	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page <= 0 {
		page = 1
	}

	skip := (int64(page) - 1) * int64(size)

	ascendingValue := 1
	if ascending != nil && !*ascending {
		ascendingValue = -1
	}

	// Build base filter
	filter := bson.M{}
	if len(titleIds) > 0 {
		filter["_id"] = bson.M{"$in": titleIds}
	}

	// Count total
	totalResults, err := db.CountTotalTitles(ctx, filter)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	////////////////////////////////////////////////////////////////////////////
	//  🟦 CASE 1 — MUST USE CUSTOM ORDER (group fields sorting)
	////////////////////////////////////////////////////////////////////////////
	groupFieldsSort := orderByField == "watched" || orderByField == "watchedAt" || orderByField == "addedAt"
	if groupFieldsSort {
		idsAsInterfaces := make([]interface{}, len(titleIds))
		for i, id := range titleIds {
			idsAsInterfaces[i] = id
		}

		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: filter}},
			{{Key: "$addFields", Value: bson.M{
				"sortOrder": bson.M{"$indexOfArray": []interface{}{idsAsInterfaces, "$_id"}},
			}}},
			{{Key: "$sort", Value: bson.M{"sortOrder": 1}}},
			{{Key: "$skip", Value: skip}},
			{{Key: "$limit", Value: int64(size)}},
		}

		titlesDb, err := db.AggregateTitles(ctx, pipeline)
		if err != nil {
			return generics.Page[Title]{}, err
		}

		titles := make([]Title, len(titlesDb))
		for i, t := range titlesDb {
			titles[i] = MapDbTitleToApiTitle(t)
		}

		return generics.Page[Title]{
			TotalResults: totalResults,
			Size:         size,
			Page:         page,
			TotalPages:   int((totalResults + size - 1) / size),
			Content:      titles,
		}, nil
	}

	////////////////////////////////////////////////////////////////////////////
	//  🟩 CASE 2 — STANDARD MONGO SORTING (no group fields sorting)
	////////////////////////////////////////////////////////////////////////////
	if orderByField == "" {
		orderByField = "primaryTitle"
	}
	if orderByField == "imdbRating" {
		orderByField = "rating.aggregateRating"
	}

	opts := options.Find().
		SetLimit(int64(size)).
		SetSkip(skip).
		SetSort(bson.D{{Key: orderByField, Value: ascendingValue}})

	dbTitles, err := db.GetTitles(ctx, filter, opts)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	titles := make([]Title, len(dbTitles))
	for i, t := range dbTitles {
		titles[i] = MapDbTitleToApiTitle(t)
	}

	return generics.Page[Title]{
		TotalResults: totalResults,
		Size:         size,
		Page:         page,
		TotalPages:   int((totalResults + size - 1) / size),
		Content:      titles,
	}, nil
}

func MapDbTitleToApiTitle(title mongodb.TitleDb) Title {
	directorNames := make([]string, len(title.Directors))
	for i, director := range title.Directors {
		directorNames[i] = director.DisplayName
	}

	writerNames := make([]string, len(title.Writers))
	for i, writer := range title.Writers {
		writerNames[i] = writer.DisplayName
	}

	starNames := make([]string, len(title.Stars))
	for i, star := range title.Stars {
		starNames[i] = star.DisplayName
	}

	originCountries := make([]string, len(title.OriginCountries))
	for i, country := range title.OriginCountries {
		originCountries[i] = country.Name
	}

	return Title{
		Id:           title.ID,
		Type:         title.Type,
		PrimaryTitle: title.PrimaryTitle,
		PrimaryImage: Image{
			URL:    title.PrimaryImage.URL,
			Width:  title.PrimaryImage.Width,
			Height: title.PrimaryImage.Height,
		},
		StartYear:      title.StartYear,
		RuntimeSeconds: title.RuntimeSeconds,
		Genres:         title.Genres,
		Rating: Rating{
			AggregateRating: title.Rating.AggregateRating,
			VoteCount:       title.Rating.VoteCount,
		},
		Plot:            title.Plot,
		DirectorsNames:  directorNames,
		WritersNames:    writerNames,
		StarsNames:      starNames,
		OriginCountries: originCountries,
		AddedAt:         title.AddedAt,
		UpdatedAt:       title.UpdatedAt,
	}
}

func AddNewTitle(db *mongodb.DB, ctx context.Context, titleId string) (Title, error) {
	// TODO: Handle the case where the titles id is returning nothing from IMDB API

	body, err := imdb.FetchTitle(titleId)
	if err != nil {
		return Title{}, err
	}

	var title mongodb.TitleDb
	if err := json.Unmarshal(body, &title); err != nil {
		return Title{}, err
	}

	// Set missing fields
	now := time.Now()
	title.AddedAt = &now
	title.UpdatedAt = &now

	doc, err := bson.Marshal(title)
	if err != nil {
		return Title{}, err
	}

	var bsonDoc bson.M
	if err := bson.Unmarshal(doc, &bsonDoc); err != nil {
		return Title{}, err
	}

	if err := db.AddTitle(ctx, bsonDoc); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return Title{}, err
		}
		// If duplicate, try to read back the stored document
		if stored, gerr := db.TitleExists(ctx, titleId); gerr == nil && stored {
			raw, _ := json.Marshal(stored)
			_ = json.Unmarshal(raw, &title)
		}
	}

	return MapDbTitleToApiTitle(title), nil
}

func UpdateTitleWatchedProperties(
	db *mongodb.DB,
	ctx context.Context,
	titleId string,
	req SetWatchedRequest,
) (Title, error) {
	titleUpdatedBd, err := db.UpdateTitleWatchedProperties(ctx, titleId, req.Watched, req.WatchedAt)
	if err != nil {
		return Title{}, err
	}

	return MapDbTitleToApiTitle(*titleUpdatedBd), nil
}

// Deletes the title and its related ratings and comments
func CascadeDeleteTitle(db *mongodb.DB, ctx context.Context, titleId string) (int64, error) {
	deletedRatingsCount, err := db.DeleteRatingsByTitleId(ctx, titleId)
	if err != nil {
		return 0, err
	}

	deletedCommentsCount, err := db.DeleteCommentsByTitleId(ctx, titleId)
	if err != nil {
		return 0, err
	}

	_, err = db.DeleteTitle(ctx, titleId)
	if err != nil {
		return 0, err
	}

	return deletedRatingsCount + deletedCommentsCount, nil

}

func GetTitleById(db *mongodb.DB, ctx context.Context, titleId string) (Title, error) {
	titleDb, err := db.GetTitleById(ctx, titleId)
	if err != nil {
		return Title{}, err
	}

	return MapDbTitleToApiTitle(titleDb), nil
}
