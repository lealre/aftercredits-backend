package titles

import (
	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
)

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

func MapImdbSeasonsToDbSeasons(seasons imdb.SeasonsResponse) []mongodb.Season {
	dbSeasons := make([]mongodb.Season, len(seasons.Seasons))
	for i, season := range seasons.Seasons {
		dbSeasons[i] = mongodb.Season{
			Season:       season.Season,
			EpisodeCount: season.EpisodeCount,
		}
	}
	return dbSeasons
}

func MapImdbEpisodesToDbEpisodes(episodes imdb.EpisodesResponse) []mongodb.Episode {
	dbEpisodes := make([]mongodb.Episode, len(episodes.Episodes))
	for i, episode := range episodes.Episodes {
		dbEpisodes[i] = mongodb.Episode{
			ID:    episode.ID,
			Title: episode.Title,
			PrimaryImage: mongodb.Image{
				URL:    episode.PrimaryImage.URL,
				Width:  episode.PrimaryImage.Width,
				Height: episode.PrimaryImage.Height,
			},
			Season:        episode.Season,
			EpisodeNumber: episode.EpisodeNumber,
		}
	}
	return dbEpisodes
}
