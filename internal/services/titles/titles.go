package titles

import (
	"context"
	"errors"
	"time"

	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
This can filter by the native titles fields from titles Ids or by the group fields sorting.

IMPORTANT: Using this method with titleIds set to nil is specifically intended to return all
titles from the titles collection, for use in the admin scenario.

🟦 CASE 1: Filter by the fields in group titles, by preserving the order in titleIds list
  - watched
  - watchedAt
  - addedAt

🟩 CASE 2: Filter by the native titles collection fields
*/
func GetPageOfTitles(
	db *mongodb.DB,
	ctx context.Context,
	size, page int,
	orderByField string,
	ascending *bool,
	titleIds []string,
) (generics.Page[Title], error) {

	if size <= 0 {
		size = config.DefaultPageSize()
	}
	if maxSize := config.MaxPageSize(); size > maxSize {
		size = maxSize
	}
	if page <= 0 {
		page = 1
	}

	skip := (int64(page) - 1) * int64(size)

	ascendingValue := 1
	if ascending != nil && !*ascending {
		ascendingValue = -1
	}

	if titleIds != nil && len(titleIds) == 0 {
		// Empty list provided explicitly - return no results
		return generics.Page[Title]{
			TotalResults: 0,
			Size:         size,
			Page:         page,
			TotalPages:   0,
			Content:      []Title{},
		}, nil
	}

	filter := bson.M{}
	if len(titleIds) > 0 {
		filter["_id"] = bson.M{"$in": titleIds}
	}

	totalResults, err := db.CountTotalTitles(ctx, filter)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	////////////////////////////////////////////////////////////////////////////
	//  🟦 CASE 1 — MUST USE CUSTOM ORDER (group fields sorting)
	////////////////////////////////////////////////////////////////////////////
	groupFieldsSort := orderByField == "watched" || orderByField == "watchedAt" || orderByField == "addedAt"
	if len(titleIds) > 0 && groupFieldsSort {
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

func AddNewTitle(db *mongodb.DB, provider titleprovider.Provider, ctx context.Context, titleId string) (Title, error) {
	logger := logx.FromContext(ctx)

	providerTitle, err := provider.GetTitle(ctx, titleId)
	if err != nil {
		// Translate the provider's not-found into the titles service vocabulary
		// so handlers map it via titles.ErrorMap (not the provider's error).
		if errors.Is(err, titleprovider.ErrTitleNotFound) {
			return Title{}, ErrTitleNotFound
		}
		return Title{}, err
	}
	if providerTitle.Type == "tvSeries" || providerTitle.Type == "tvMiniSeries" {
		logger.Printf("Title %s is a TV series with %d seasons", titleId, len(providerTitle.Seasons))
	}

	title := MapProviderTitleToDb(*providerTitle)

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
		// If duplicate, read back the stored document
		if stored, gerr := db.GetTitleById(ctx, titleId); gerr == nil {
			title = stored
		}
	}

	return MapDbTitleToApiTitle(title), nil
}

func DeleteTitle(db *mongodb.DB, ctx context.Context, titleId string) error {
	_, err := db.DeleteTitle(ctx, titleId)
	if err != nil {
		return err
	}

	return nil
}

func GetTitleById(db *mongodb.DB, ctx context.Context, titleId string) (Title, error) {
	titleDb, err := db.GetTitleById(ctx, titleId)
	if err != nil {
		return Title{}, err
	}

	return MapDbTitleToApiTitle(titleDb), nil
}

func SearchTitles(provider titleprovider.Provider, ctx context.Context, searchQuery string, limit int) ([]Title, error) {
	items, err := provider.SearchTitles(ctx, searchQuery, limit)
	if err != nil {
		return nil, err
	}
	return MapProviderSearchItemsToTitles(items), nil
}
