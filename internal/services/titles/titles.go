package titles

import (
	"context"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
)

// mapTitleToMovie converts an imdb.Title to api.Movie
func MapImdbTitleToDbTitle(title imdb.Title) Title {
	// Extract director names
	directorNames := make([]string, len(title.Directors))
	for i, director := range title.Directors {
		directorNames[i] = director.DisplayName
	}

	// Extract writer names
	writerNames := make([]string, len(title.Writers))
	for i, writer := range title.Writers {
		writerNames[i] = writer.DisplayName
	}

	// Extract star names
	starNames := make([]string, len(title.Stars))
	for i, star := range title.Stars {
		starNames[i] = star.DisplayName
	}

	// Extract origin country names
	originCountries := make([]string, len(title.OriginCountries))
	for i, country := range title.OriginCountries {
		originCountries[i] = country.Name
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

// func GetTitleRatings(ctx context.Context, id string) ([]ratings.Rating, error) {
// 	coll := mongodb.GetRatingsCollection(ctx)
// 	cursor, err := coll.Find(ctx, bson.M{"titleId": id})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return cursor, nil
// }
