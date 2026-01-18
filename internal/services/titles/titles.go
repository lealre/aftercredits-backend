package titles

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
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

func AddNewTitle(db *mongodb.DB, ctx context.Context, titleId string) (Title, error) {
	// TODO: Handle the case where the titles id is returning nothing from IMDB API
	logger := logx.FromContext(ctx)

	body, err := imdb.FetchTitle(titleId)
	if err != nil {
		return Title{}, err
	}

	var title mongodb.TitleDb
	if err := json.Unmarshal(body, &title); err != nil {
		return Title{}, err
	}

	// If title is a TV series, fetch seasons/episodes from IMDB (for validation purposes)
	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Title %s is a TV series, fetching seasons/episodes from IMDB", titleId)
		seasonsBody, err := imdb.FetchSeasons(titleId)
		if err != nil {
			return Title{}, err
		}

		var seasonsResp imdb.SeasonsResponse
		if err := json.Unmarshal(seasonsBody, &seasonsResp); err != nil {
			return Title{}, err
		}

		episodesBody, err := imdb.FetchEpisodes(titleId)
		if err != nil {
			return Title{}, err
		}

		var episodesResp imdb.EpisodesResponse
		if err := json.Unmarshal(episodesBody, &episodesResp); err != nil {
			return Title{}, err
		}

		logger.Printf("Seasons: %v", seasonsResp)
		logger.Printf("Episodes: %v", episodesResp)

		title.Seasons = MapImdbSeasonsToDbSeasons(seasonsResp.Seasons)
		title.Episodes = MapImdbEpisodesToDbEpisodes(episodesResp.Episodes)
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
