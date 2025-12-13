package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// ----- Methods for the database -----

func (db *DB) GetTitleById(ctx context.Context, id string) (TitleDb, error) {
	coll := db.Collection(TitlesCollection)
	var titleDb TitleDb
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&titleDb); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return TitleDb{}, ErrRecordNotFound
		}
		return TitleDb{}, err
	}
	return titleDb, nil
}

func (db *DB) AddTitle(ctx context.Context, doc map[string]any) error {
	if doc == nil {
		return fmt.Errorf("doc is nil")
	}
	if _, ok := doc["_id"]; !ok {
		return fmt.Errorf("doc missing _id")
	}
	coll := db.Collection(TitlesCollection)
	_, err := coll.InsertOne(ctx, doc)
	return err
}

func (db *DB) DeleteTitle(ctx context.Context, id string) (bool, error) {
	coll := db.Collection(TitlesCollection)
	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func (db *DB) GetTitles(ctx context.Context, args ...any) ([]TitleDb, error) {
	coll := db.Collection(TitlesCollection)

	filter, opts := ResolveFilterAndOptionsSearch(args...)
	cursor, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var allTitles []TitleDb
	if err := cursor.All(ctx, &allTitles); err != nil {
		return []TitleDb{}, err
	}

	return allTitles, nil
}

func (db *DB) CountTotalTitles(ctx context.Context, args ...any) (int, error) {
	coll := db.Collection(TitlesCollection)

	filter, _ := ResolveFilterAndOptionsSearch(args...)
	totalTitles, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(totalTitles), nil
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

func (db *DB) AggregateTitles(ctx context.Context, pipeline mongo.Pipeline) ([]TitleDb, error) {
	coll := db.Collection(TitlesCollection)

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return []TitleDb{}, err
	}
	defer cursor.Close(ctx)

	var dbTitles []TitleDb
	if err := cursor.All(ctx, &dbTitles); err != nil {
		return []TitleDb{}, err
	}

	return dbTitles, nil
}
