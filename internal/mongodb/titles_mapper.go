package mongodb

import "github.com/lealre/movies-backend/internal/models"

// titleDbToModel converts the mongo-specific TitleDb into the storage-neutral
// models.Title used by the service layer.
func titleDbToModel(t TitleDb) models.Title {
	return models.Title{
		ID:              t.ID,
		Type:            t.Type,
		PrimaryTitle:    t.PrimaryTitle,
		PrimaryImage:    imageDbToModel(t.PrimaryImage),
		StartYear:       t.StartYear,
		RuntimeSeconds:  t.RuntimeSeconds,
		Genres:          t.Genres,
		Rating:          ratingDbToModel(t.Rating),
		Metacritic:      metacriticDbToModel(t.Metacritic),
		Plot:            t.Plot,
		Directors:       personsDbToModel(t.Directors),
		Writers:         personsDbToModel(t.Writers),
		Stars:           personsDbToModel(t.Stars),
		OriginCountries: codeNamesDbToModel(t.OriginCountries),
		SpokenLanguages: codeNamesDbToModel(t.SpokenLanguages),
		Interests:       interestsDbToModel(t.Interests),
		Seasons:         seasonsDbToModel(t.Seasons),
		Episodes:        episodesDbToModel(t.Episodes),
		AddedAt:         t.AddedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// titleModelToDb converts a storage-neutral models.Title back into the
// mongo-specific TitleDb used at the persistence boundary.
func titleModelToDb(t models.Title) TitleDb {
	return TitleDb{
		ID:              t.ID,
		Type:            t.Type,
		PrimaryTitle:    t.PrimaryTitle,
		PrimaryImage:    imageModelToDb(t.PrimaryImage),
		StartYear:       t.StartYear,
		RuntimeSeconds:  t.RuntimeSeconds,
		Genres:          t.Genres,
		Rating:          ratingModelToDb(t.Rating),
		Metacritic:      metacriticModelToDb(t.Metacritic),
		Plot:            t.Plot,
		Directors:       personsModelToDb(t.Directors),
		Writers:         personsModelToDb(t.Writers),
		Stars:           personsModelToDb(t.Stars),
		OriginCountries: codeNamesModelToDb(t.OriginCountries),
		SpokenLanguages: codeNamesModelToDb(t.SpokenLanguages),
		Interests:       interestsModelToDb(t.Interests),
		Seasons:         seasonsModelToDb(t.Seasons),
		Episodes:        episodesModelToDb(t.Episodes),
		AddedAt:         t.AddedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// ----- Image -----

func imageDbToModel(i Image) models.Image {
	return models.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func imageModelToDb(i models.Image) Image {
	return Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func imagePtrDbToModel(i *Image) *models.Image {
	if i == nil {
		return nil
	}
	m := imageDbToModel(*i)
	return &m
}

func imagePtrModelToDb(i *models.Image) *Image {
	if i == nil {
		return nil
	}
	d := imageModelToDb(*i)
	return &d
}

// ----- Rating -----

func ratingDbToModel(r Rating) models.Rating {
	return models.Rating{AggregateRating: r.AggregateRating, VoteCount: r.VoteCount}
}

func ratingModelToDb(r models.Rating) Rating {
	return Rating{AggregateRating: r.AggregateRating, VoteCount: r.VoteCount}
}

func ratingPtrDbToModel(r *Rating) *models.Rating {
	if r == nil {
		return nil
	}
	m := ratingDbToModel(*r)
	return &m
}

func ratingPtrModelToDb(r *models.Rating) *Rating {
	if r == nil {
		return nil
	}
	d := ratingModelToDb(*r)
	return &d
}

// ----- Metacritic -----

func metacriticDbToModel(m *Metacritic) *models.Metacritic {
	if m == nil {
		return nil
	}
	return &models.Metacritic{Score: m.Score, ReviewCount: m.ReviewCount}
}

func metacriticModelToDb(m *models.Metacritic) *Metacritic {
	if m == nil {
		return nil
	}
	return &Metacritic{Score: m.Score, ReviewCount: m.ReviewCount}
}

// ----- Person -----

func personDbToModel(p Person) models.Person {
	return models.Person{
		ID:                 p.ID,
		DisplayName:        p.DisplayName,
		AlternativeNames:   p.AlternativeNames,
		PrimaryImage:       imagePtrDbToModel(p.PrimaryImage),
		PrimaryProfessions: p.PrimaryProfessions,
	}
}

func personModelToDb(p models.Person) Person {
	return Person{
		ID:                 p.ID,
		DisplayName:        p.DisplayName,
		AlternativeNames:   p.AlternativeNames,
		PrimaryImage:       imagePtrModelToDb(p.PrimaryImage),
		PrimaryProfessions: p.PrimaryProfessions,
	}
}

func personsDbToModel(ps []Person) []models.Person {
	out := make([]models.Person, len(ps))
	for i, p := range ps {
		out[i] = personDbToModel(p)
	}
	return out
}

func personsModelToDb(ps []models.Person) []Person {
	out := make([]Person, len(ps))
	for i, p := range ps {
		out[i] = personModelToDb(p)
	}
	return out
}

// ----- CodeName -----

func codeNameDbToModel(c CodeName) models.CodeName {
	return models.CodeName{Code: c.Code, Name: c.Name}
}

func codeNameModelToDb(c models.CodeName) CodeName {
	return CodeName{Code: c.Code, Name: c.Name}
}

func codeNamesDbToModel(cs []CodeName) []models.CodeName {
	out := make([]models.CodeName, len(cs))
	for i, c := range cs {
		out[i] = codeNameDbToModel(c)
	}
	return out
}

func codeNamesModelToDb(cs []models.CodeName) []CodeName {
	out := make([]CodeName, len(cs))
	for i, c := range cs {
		out[i] = codeNameModelToDb(c)
	}
	return out
}

// ----- Interest -----

func interestDbToModel(i Interest) models.Interest {
	return models.Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre}
}

func interestModelToDb(i models.Interest) Interest {
	return Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre}
}

func interestsDbToModel(is []Interest) []models.Interest {
	out := make([]models.Interest, len(is))
	for i, v := range is {
		out[i] = interestDbToModel(v)
	}
	return out
}

func interestsModelToDb(is []models.Interest) []Interest {
	out := make([]Interest, len(is))
	for i, v := range is {
		out[i] = interestModelToDb(v)
	}
	return out
}

// ----- Seasons -----

func seasonDbToModel(s Seasons) models.Seasons {
	return models.Seasons{Season: s.Season, EpisodeCount: s.EpisodeCount}
}

func seasonModelToDb(s models.Seasons) Seasons {
	return Seasons{Season: s.Season, EpisodeCount: s.EpisodeCount}
}

func seasonsDbToModel(ss []Seasons) []models.Seasons {
	out := make([]models.Seasons, len(ss))
	for i, s := range ss {
		out[i] = seasonDbToModel(s)
	}
	return out
}

func seasonsModelToDb(ss []models.Seasons) []Seasons {
	out := make([]Seasons, len(ss))
	for i, s := range ss {
		out[i] = seasonModelToDb(s)
	}
	return out
}

// ----- ReleaseDate -----

func releaseDateDbToModel(r *ReleaseDate) *models.ReleaseDate {
	if r == nil {
		return nil
	}
	return &models.ReleaseDate{Year: r.Year, Month: r.Month, Day: r.Day}
}

func releaseDateModelToDb(r *models.ReleaseDate) *ReleaseDate {
	if r == nil {
		return nil
	}
	return &ReleaseDate{Year: r.Year, Month: r.Month, Day: r.Day}
}

// ----- Episode -----

func episodeDbToModel(e Episode) models.Episode {
	return models.Episode{
		ID:             e.ID,
		Title:          e.Title,
		PrimaryImage:   imageDbToModel(e.PrimaryImage),
		Season:         e.Season,
		EpisodeNumber:  e.EpisodeNumber,
		RuntimeSeconds: e.RuntimeSeconds,
		Plot:           e.Plot,
		Rating:         ratingPtrDbToModel(e.Rating),
		ReleaseDate:    releaseDateDbToModel(e.ReleaseDate),
	}
}

func episodeModelToDb(e models.Episode) Episode {
	return Episode{
		ID:             e.ID,
		Title:          e.Title,
		PrimaryImage:   imageModelToDb(e.PrimaryImage),
		Season:         e.Season,
		EpisodeNumber:  e.EpisodeNumber,
		RuntimeSeconds: e.RuntimeSeconds,
		Plot:           e.Plot,
		Rating:         ratingPtrModelToDb(e.Rating),
		ReleaseDate:    releaseDateModelToDb(e.ReleaseDate),
	}
}

func episodesDbToModel(es []Episode) []models.Episode {
	out := make([]models.Episode, len(es))
	for i, e := range es {
		out[i] = episodeDbToModel(e)
	}
	return out
}

func episodesModelToDb(es []models.Episode) []Episode {
	out := make([]Episode, len(es))
	for i, e := range es {
		out[i] = episodeModelToDb(e)
	}
	return out
}
