package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ----- Types for the database -----

type TitleDb struct {
	ID              string      `json:"id" bson:"_id"`
	Type            string      `json:"type" bson:"type"`
	PrimaryTitle    string      `json:"primaryTitle" bson:"primaryTitle"`
	PrimaryImage    Image       `json:"primaryImage" bson:"primaryImage"`
	StartYear       int         `json:"startYear" bson:"startYear"`
	RuntimeSeconds  int         `json:"runtimeSeconds" bson:"runtimeSeconds"`
	Genres          []string    `json:"genres" bson:"genres"`
	Rating          Rating      `json:"rating" bson:"rating"`
	Metacritic      *Metacritic `json:"metacritic,omitempty" bson:"metacritic,omitempty"`
	Plot            string      `json:"plot" bson:"plot"`
	Directors       []Person    `json:"directors" bson:"directors"`
	Writers         []Person    `json:"writers" bson:"writers"`
	Stars           []Person    `json:"stars" bson:"stars"`
	OriginCountries []CodeName  `json:"originCountries" bson:"originCountries"`
	SpokenLanguages []CodeName  `json:"spokenLanguages" bson:"spokenLanguages"`
	Interests       []Interest  `json:"interests" bson:"interests"`
	Seasons         []Seasons   `json:"seasons" bson:"seasons"`
	Episodes        []Episode   `json:"episodes" bson:"episodes"`
	AddedAt         *time.Time  `json:"addedAt,omitempty" bson:"addedAt,omitempty"`
	UpdatedAt       *time.Time  `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

type Image struct {
	URL    string `json:"url" bson:"url"`
	Width  int    `json:"width" bson:"width"`
	Height int    `json:"height" bson:"height"`
}

type Person struct {
	ID                 string   `json:"id" bson:"id"`
	DisplayName        string   `json:"displayName" bson:"displayName"`
	AlternativeNames   []string `json:"alternativeNames,omitempty" bson:"alternativeNames,omitempty"`
	PrimaryImage       *Image   `json:"primaryImage,omitempty" bson:"primaryImage,omitempty"`
	PrimaryProfessions []string `json:"primaryProfessions,omitempty" bson:"primaryProfessions,omitempty"`
}

type Rating struct {
	AggregateRating float64 `json:"aggregateRating" bson:"aggregateRating"`
	VoteCount       int     `json:"voteCount" bson:"voteCount"`
}

type Metacritic struct {
	Score       int `json:"score" bson:"score"`
	ReviewCount int `json:"reviewCount" bson:"reviewCount"`
}

type CodeName struct {
	Code string `json:"code" bson:"code"`
	Name string `json:"name" bson:"name"`
}

type Interest struct {
	ID         string `json:"id" bson:"id"`
	Name       string `json:"name" bson:"name"`
	IsSubgenre bool   `json:"isSubgenre,omitempty" bson:"isSubgenre,omitempty"`
}

type Seasons struct {
	Season       string `json:"season" bson:"season"`
	EpisodeCount int    `json:"episodeCount" bson:"episodeCount"`
}

type Episode struct {
	ID             string       `json:"id" bson:"id"`
	Title          string       `json:"title" bson:"title"`
	PrimaryImage   Image        `json:"primaryImage" bson:"primaryImage"`
	Season         string       `json:"season" bson:"season"`
	EpisodeNumber  int          `json:"episodeNumber" bson:"episodeNumber"`
	RuntimeSeconds *int         `json:"runtimeSeconds,omitempty" bson:"runtimeSeconds,omitempty"`
	Plot           *string      `json:"plot,omitempty" bson:"plot,omitempty"`
	Rating         *Rating      `json:"rating,omitempty" bson:"rating,omitempty"`
	ReleaseDate    *ReleaseDate `json:"releaseDate,omitempty" bson:"releaseDate,omitempty"`
}

type ReleaseDate struct {
	Year  int `json:"year" bson:"year"`
	Month int `json:"month" bson:"month"`
	Day   int `json:"day" bson:"day"`
}

// ----- Methods for the database -----

func (db *DB) GetTitleById(ctx context.Context, id string) (models.Title, error) {
	coll := db.Collection(TitlesCollection)
	var titleDb TitleDb
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&titleDb); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Title{}, store.ErrRecordNotFound
		}
		return models.Title{}, err
	}
	return titleDbToModel(titleDb), nil
}

// AddTitle inserts a title document. It takes a storage-neutral models.Title
// and maps it to the mongo-specific TitleDb internally before persisting.
func (db *DB) AddTitle(ctx context.Context, title models.Title) error {
	if title.ID == "" {
		return fmt.Errorf("title missing id")
	}
	coll := db.Collection(TitlesCollection)
	_, err := coll.InsertOne(ctx, titleModelToDb(title))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	return nil
}

func (db *DB) DeleteTitle(ctx context.Context, id string) (bool, error) {
	coll := db.Collection(TitlesCollection)
	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// GetTitlesPage is the single Mongo-specific entry point for paginated title
// listing. It owns every Mongo-only concern that used to leak into the
// titles service: filter/pipeline construction, the two sort strategies
// (custom ids-order for group-field sorting vs. standard Mongo sort), the
// Mongo-specific orderBy field remapping ("" -> primaryTitle, imdbRating ->
// rating.aggregateRating), and the total-count query. size and page are
// expected to already be normalized (positive) by the caller; ids and
// orderBy/ascending are passed through as-is.
//
// 🟦 CASE 1: when ids is non-empty and orderBy is one of the group-title
// fields (watched/watchedAt/addedAt), results are sorted by the position of
// each id within ids (preserving the caller-supplied order) via an
// aggregation pipeline.
//
// 🟩 CASE 2: standard Mongo find+sort on the (possibly remapped) orderBy
// field.
func (db *DB) GetTitlesPage(
	ctx context.Context,
	ids []string,
	orderBy string,
	ascending *bool,
	size, page int,
) ([]models.Title, int64, error) {
	coll := db.Collection(TitlesCollection)

	if ids != nil && len(ids) == 0 {
		// Empty list provided explicitly - return no results
		return []models.Title{}, 0, nil
	}

	skip := (int64(page) - 1) * int64(size)

	ascendingValue := 1
	if ascending != nil && !*ascending {
		ascendingValue = -1
	}

	filter := bson.M{}
	if len(ids) > 0 {
		filter["_id"] = bson.M{"$in": ids}
	}

	totalResults, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	groupFieldsSort := orderBy == "watched" || orderBy == "watchedAt" || orderBy == "addedAt"
	if len(ids) > 0 && groupFieldsSort {
		idsAsInterfaces := make([]interface{}, len(ids))
		for i, id := range ids {
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

		cursor, err := coll.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, 0, err
		}
		defer cursor.Close(ctx)

		var dbTitles []TitleDb
		if err := cursor.All(ctx, &dbTitles); err != nil {
			return nil, 0, err
		}

		titles := make([]models.Title, len(dbTitles))
		for i, t := range dbTitles {
			titles[i] = titleDbToModel(t)
		}

		return titles, totalResults, nil
	}

	if orderBy == "" {
		orderBy = "primaryTitle"
	}
	if orderBy == "imdbRating" {
		orderBy = "rating.aggregateRating"
	}

	opts := options.Find().
		SetLimit(int64(size)).
		SetSkip(skip).
		SetSort(bson.D{{Key: orderBy, Value: ascendingValue}})

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var allTitlesDb []TitleDb
	if err := cursor.All(ctx, &allTitlesDb); err != nil {
		return nil, 0, err
	}

	titles := make([]models.Title, len(allTitlesDb))
	for i, t := range allTitlesDb {
		titles[i] = titleDbToModel(t)
	}

	return titles, totalResults, nil
}

func (db *DB) TitleExists(ctx context.Context, id string) (bool, error) {
	coll := db.Collection(TitlesCollection)

	// Only ask MongoDB for the _id field
	opts := options.FindOne().SetProjection(bson.M{"_id": 1})

	err := coll.FindOne(ctx, bson.M{"_id": id}, opts).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetTitleTypes fetches title types from the database for the given title IDs.
// Returns a map of titleId -> type.
func (db *DB) GetTitleTypes(ctx context.Context, titleIds []string) (map[string]string, error) {
	if len(titleIds) == 0 {
		return make(map[string]string), nil
	}

	coll := db.Collection(TitlesCollection)

	// Use projection to fetch only _id and type fields
	projection := bson.M{
		"_id":  1,
		"type": 1,
	}

	filter := bson.M{
		"_id": bson.M{"$in": titleIds},
	}

	opts := options.Find().SetProjection(projection)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	titleTypes := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			ID   string `bson:"_id"`
			Type string `bson:"type"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		titleTypes[doc.ID] = doc.Type
	}

	return titleTypes, cursor.Err()
}
