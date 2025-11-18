package titles

import "github.com/lealre/movies-backend/internal/mongodb"

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
