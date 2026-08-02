package titles

import (
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

func MapDbTitleToApiTitle(title models.Title) Title {
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
		Seasons:         MapDbSeasonsToImdbSeasons(title.Seasons),
		Episodes:        MapDbEpisodesToImdbEpisodes(title.Episodes),
		OriginCountries: originCountries,
		AddedAt:         title.AddedAt,
		UpdatedAt:       title.UpdatedAt,
	}
}

func MapImdbSeasonsToDbSeasons(seasons []titleprovider.Season) []models.Seasons {
	dbSeasons := make([]models.Seasons, len(seasons))
	for i, season := range seasons {
		dbSeasons[i] = models.Seasons{
			Season:       season.Season,
			EpisodeCount: season.EpisodeCount,
		}
	}
	return dbSeasons
}

func MapImdbEpisodesToDbEpisodes(episodes []titleprovider.Episode) []models.Episode {
	dbEpisodes := make([]models.Episode, len(episodes))
	for i, episode := range episodes {
		dbEpisode := models.Episode{
			ID:    episode.ID,
			Title: episode.Title,
			PrimaryImage: models.Image{
				URL:    episode.PrimaryImage.URL,
				Width:  episode.PrimaryImage.Width,
				Height: episode.PrimaryImage.Height,
			},
			Season:        episode.Season,
			EpisodeNumber: episode.EpisodeNumber,
		}

		// Map optional fields
		if episode.RuntimeSeconds != nil {
			dbEpisode.RuntimeSeconds = episode.RuntimeSeconds
		}

		if episode.Plot != nil {
			dbEpisode.Plot = episode.Plot
		}

		if episode.Rating != nil {
			dbEpisode.Rating = &models.Rating{
				AggregateRating: episode.Rating.AggregateRating,
				VoteCount:       episode.Rating.VoteCount,
			}
		}

		if episode.ReleaseDate != nil {
			dbEpisode.ReleaseDate = &models.ReleaseDate{
				Year:  episode.ReleaseDate.Year,
				Month: episode.ReleaseDate.Month,
				Day:   episode.ReleaseDate.Day,
			}
		}

		dbEpisodes[i] = dbEpisode
	}
	return dbEpisodes
}

func MapDbSeasonsToImdbSeasons(seasons []models.Seasons) []Seasons {
	imdbSeasons := make([]Seasons, len(seasons))
	for i, season := range seasons {
		imdbSeasons[i] = Seasons{
			Season:       season.Season,
			EpisodeCount: season.EpisodeCount,
		}
	}
	return imdbSeasons
}

func MapDbEpisodesToImdbEpisodes(episodes []models.Episode) []Episode {
	apiEpisodes := make([]Episode, len(episodes))
	for i, episode := range episodes {
		apiEpisode := Episode{
			ID:    episode.ID,
			Title: episode.Title,
			PrimaryImage: Image{
				URL:    episode.PrimaryImage.URL,
				Width:  episode.PrimaryImage.Width,
				Height: episode.PrimaryImage.Height,
			},
			Season:        episode.Season,
			EpisodeNumber: episode.EpisodeNumber,
		}

		if episode.RuntimeSeconds != nil {
			apiEpisode.RuntimeSeconds = episode.RuntimeSeconds
		}

		if episode.Plot != nil {
			apiEpisode.Plot = episode.Plot
		}

		if episode.Rating != nil {
			apiEpisode.Rating = &Rating{
				AggregateRating: episode.Rating.AggregateRating,
				VoteCount:       episode.Rating.VoteCount,
			}
		}

		if episode.ReleaseDate != nil {
			apiEpisode.ReleaseDate = &ReleaseDate{
				Year:  episode.ReleaseDate.Year,
				Month: episode.ReleaseDate.Month,
				Day:   episode.ReleaseDate.Day,
			}
		}

		apiEpisodes[i] = apiEpisode
	}
	return apiEpisodes
}
