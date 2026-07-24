package titles

import (
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// MapProviderTitleToDb converts a provider-neutral Title into the MongoDB
// document shape. This replaces the previous direct json.Unmarshal into TitleDb.
func MapProviderTitleToDb(t titleprovider.Title) mongodb.TitleDb {
	db := mongodb.TitleDb{
		ID:             t.ID,
		Type:           t.Type,
		PrimaryTitle:   t.PrimaryTitle,
		PrimaryImage:   mapProviderImage(t.PrimaryImage),
		StartYear:      t.StartYear,
		RuntimeSeconds: t.RuntimeSeconds,
		Genres:         t.Genres,
		Rating:         mongodb.Rating{AggregateRating: t.Rating.AggregateRating, VoteCount: t.Rating.VoteCount},
		Plot:           t.Plot,
		Directors:      mapProviderPersons(t.Directors),
		Writers:        mapProviderPersons(t.Writers),
		Stars:          mapProviderPersons(t.Stars),
		Seasons:        mapProviderSeasons(t.Seasons),
		Episodes:       mapProviderEpisodes(t.Episodes),
	}
	if t.Metacritic != nil {
		db.Metacritic = &mongodb.Metacritic{Score: t.Metacritic.Score, ReviewCount: t.Metacritic.ReviewCount}
	}
	for _, c := range t.OriginCountries {
		db.OriginCountries = append(db.OriginCountries, mongodb.CodeName{Code: c.Code, Name: c.Name})
	}
	for _, l := range t.SpokenLanguages {
		db.SpokenLanguages = append(db.SpokenLanguages, mongodb.CodeName{Code: l.Code, Name: l.Name})
	}
	for _, i := range t.Interests {
		db.Interests = append(db.Interests, mongodb.Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre})
	}
	return db
}

// MapProviderSearchItemsToTitles converts provider search items into the
// service-layer Title used in search responses.
func MapProviderSearchItemsToTitles(items []titleprovider.SearchItem) []Title {
	out := make([]Title, len(items))
	for i, it := range items {
		out[i] = Title{
			Id:           it.ID,
			Type:         it.Type,
			PrimaryTitle: it.PrimaryTitle,
			PrimaryImage: Image{URL: it.PrimaryImage.URL, Width: it.PrimaryImage.Width, Height: it.PrimaryImage.Height},
			StartYear:    it.StartYear,
			Rating:       Rating{AggregateRating: it.Rating.AggregateRating, VoteCount: it.Rating.VoteCount},
		}
	}
	return out
}

func mapProviderImage(i titleprovider.Image) mongodb.Image {
	return mongodb.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func mapProviderPersons(ps []titleprovider.Person) []mongodb.Person {
	out := make([]mongodb.Person, 0, len(ps))
	for _, p := range ps {
		person := mongodb.Person{
			ID:                 p.ID,
			DisplayName:        p.DisplayName,
			AlternativeNames:   p.AlternativeNames,
			PrimaryProfessions: p.PrimaryProfessions,
		}
		if p.PrimaryImage != nil {
			img := mapProviderImage(*p.PrimaryImage)
			person.PrimaryImage = &img
		}
		out = append(out, person)
	}
	return out
}

func mapProviderSeasons(ss []titleprovider.Season) []mongodb.Seasons {
	out := make([]mongodb.Seasons, len(ss))
	for i, s := range ss {
		out[i] = mongodb.Seasons{Season: s.Season, EpisodeCount: s.EpisodeCount}
	}
	return out
}

func mapProviderEpisodes(es []titleprovider.Episode) []mongodb.Episode {
	out := make([]mongodb.Episode, len(es))
	for i, e := range es {
		ep := mongodb.Episode{
			ID:             e.ID,
			Title:          e.Title,
			PrimaryImage:   mapProviderImage(e.PrimaryImage),
			Season:         e.Season,
			EpisodeNumber:  e.EpisodeNumber,
			RuntimeSeconds: e.RuntimeSeconds,
			Plot:           e.Plot,
		}
		if e.Rating != nil {
			ep.Rating = &mongodb.Rating{AggregateRating: e.Rating.AggregateRating, VoteCount: e.Rating.VoteCount}
		}
		if e.ReleaseDate != nil {
			ep.ReleaseDate = &mongodb.ReleaseDate{Year: e.ReleaseDate.Year, Month: e.ReleaseDate.Month, Day: e.ReleaseDate.Day}
		}
		out[i] = ep
	}
	return out
}
