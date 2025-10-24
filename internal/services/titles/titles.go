package titles

import (
	"context"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
)

func GetPageOfTitles(ctx context.Context, size, page int) (generics.Page[Title], error) {
	cursor, err := GetAllTitlesDb(ctx)
	if err != nil {
		return generics.Page[Title]{}, err
	}
	defer cursor.Close(ctx)

	var allMovies []Title

	for cursor.Next(ctx) {
		var title imdb.Title
		if err := cursor.Decode(&title); err != nil {
			return generics.Page[Title]{}, nil
		}

		movie := MapDbTitleToApiTitle(title)
		allMovies = append(allMovies, movie)
	}

	if err := cursor.Err(); err != nil {
		return generics.Page[Title]{}, nil
	}

	return generics.Page[Title]{
		TotalResults: len(allMovies),
		Page:         1,
		TotalPages:   1,
		Content:      allMovies,
	}, nil
}

// mapTitleToMovie converts an imdb.Title to api.Movie
func MapDbTitleToApiTitle(title imdb.Title) Title {
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

	// Set watched to false if it is not set
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
