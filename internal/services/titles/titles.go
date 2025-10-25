package titles

import (
	"context"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
This gets the paginated titles

Example on how to filter by field
filter := bson.M{"category": "news"}

Example on how to set limits, offsets, orderBy, ...
opts := options.Find().SetSort(bson.D{{"addedAt", -1}}).SetLimit(20)
*/
func GetPageOfTitles(ctx context.Context, size, page int, orderByField string) (generics.Page[Title], error) {
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
		orderByField = "addedAt"
	}

	skip := (int64(page) - 1) * int64(size)
	opts := options.Find().
		SetLimit(int64(size)).
		SetSkip(skip).
		SetSort(bson.D{{Key: orderByField, Value: -1}})

	totalTitlesInDB, err := CountTotalTitlesDb(ctx)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	allTitles, err := GetTitlesDb(ctx, opts)
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

// mapTitleToMovie converts an imdb.Title to api.Movie
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
	_, err := GetTitleByID(ctx, id)
	if err == nil {
		return true, nil
	}
	if err == mongodb.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}
