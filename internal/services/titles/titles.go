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

/*
This gets the paginated titles

Example on how to filter by field
filter := bson.M{"category": "news"}

Example on how to set limits, offsets, orderBy, ...
opts := options.Find().SetSort(bson.D{{"addedAt", -1}}).SetLimit(20)
*/
func GetPageOfTitles(
	ctx context.Context,
	size, page int,
	orderByField string,
	watched *bool,
	ascending *bool,
) (generics.Page[Title], error) {
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page == 0 {
		page = 1
	}
	if orderByField == "" {
		orderByField = "primaryTitle"
	}
	// Handle nested rating field
	if orderByField == "imdbRating" {
		orderByField = "rating.aggregateRating"
	}
	orderByValue := 1
	if ascending != nil {
		if !*ascending {
			orderByValue = -1
		}
	}

	skip := (int64(page) - 1) * int64(size)
	opts := options.Find().
		SetLimit(int64(size)).
		SetSkip(skip).
		SetSort(bson.D{{Key: orderByField, Value: orderByValue}})

	filter := bson.M{}
	if watched != nil {
		filter["watched"] = *watched
	}

	totalTitlesInDB, err := CountTotalTitlesDb(ctx, filter)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	allTitles, err := getTitlesDb(ctx, filter, opts)
	if err != nil {
		return generics.Page[Title]{}, err
	}
	if allTitles == nil {
		allTitles = []Title{}
	}

	totalPages := (totalTitlesInDB + size - 1) / size
	if totalTitlesInDB == 0 {
		totalPages = 1
	}

	return generics.Page[Title]{
		TotalResults: totalTitlesInDB,
		Size:         size,
		Page:         page,
		TotalPages:   int(totalPages),
		Content:      allTitles,
	}, nil
}

// mapTitleToTitle converts an imdb.Title to api.Title
func MapDbTitleToApiTitle(title imdb.Title) Title {
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

	watched := title.Watched
	if !watched {
		watched = false
	}

	return Title{
		ID:           title.ID,
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
		Watched:         watched,
		AddedAt:         title.AddedAt,
		UpdatedAt:       title.UpdatedAt,
		WatchedAt:       title.WatchedAt,
	}
}

func ChecKIfTitleExist(ctx context.Context, id string) (bool, error) {
	_, err := getTitleByIdDb(ctx, id)
	if err == nil {
		return true, nil
	}
	if err == mongodb.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}

func AddNewTitle(ctx context.Context, titleId string) (Title, error) {
	// TODO: Handle the case where the titles id is returning nothing from IMDB API

	body, err := imdb.FetchTitle(titleId)
	if err != nil {
		return Title{}, err
	}

	var title imdb.Title
	if err := json.Unmarshal(body, &title); err != nil {
		return Title{}, err
	}

	// Set missing fields
	title.Watched = false
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

	if err := addTitleDb(ctx, bsonDoc); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return Title{}, err
		}
		// If duplicate, try to read back the stored document
		if stored, gerr := ChecKIfTitleExist(ctx, titleId); gerr == nil && stored {
			raw, _ := json.Marshal(stored)
			_ = json.Unmarshal(raw, &title)
		}
	}

	return MapDbTitleToApiTitle(title), nil
}

func UpdateTitleWatchedProperties(
	ctx context.Context,
	titleId string,
	req SetWatchedRequest,
) (*Title, error) {
	return updateTitleWatchedPropertiesDb(ctx, titleId, req.Watched, req.WatchedAt)
}

func CascadeDeleteTitle(ctx context.Context, titleId string) (int64, error) {
	return cascadeDeleteTitleDb(ctx, titleId)
}
