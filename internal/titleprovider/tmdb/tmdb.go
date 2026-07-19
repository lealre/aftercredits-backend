package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

const (
	defaultBaseURL = "https://api.themoviedb.org/3"
	imageBaseURL   = "https://image.tmdb.org/t/p/original"
)

// Provider implements titleprovider.Provider against The Movie Database (TMDB) v3.
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New returns a TMDB provider using the public v3 base URL.
func New(apiKey string) *Provider {
	return &Provider{baseURL: defaultBaseURL, apiKey: apiKey, client: http.DefaultClient}
}

// newWithBaseURL is the test seam; it points the provider at a local server.
func newWithBaseURL(baseURL, apiKey string) *Provider {
	return &Provider{baseURL: baseURL, apiKey: apiKey, client: http.DefaultClient}
}

func (p *Provider) Name() string { return "tmdb" }

// getJSON performs a GET with the api_key query param and decodes JSON into out.
func (p *Provider) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", p.apiKey)
	reqURL := fmt.Sprintf("%s%s?%s", p.baseURL, path, query.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tmdb: non-2xx status %s for %s - %s", resp.Status, path, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	var find findResponse
	q := url.Values{}
	q.Set("external_source", "imdb_id")
	if err := p.getJSON(ctx, "/find/"+imdbID, q, &find); err != nil {
		return nil, err
	}

	switch {
	case len(find.MovieResults) > 0:
		return p.getMovie(ctx, imdbID, find.MovieResults[0].ID)
	case len(find.TVResults) > 0:
		return p.getTV(ctx, imdbID, find.TVResults[0].ID)
	default:
		return nil, titleprovider.ErrTitleNotFound
	}
}

func (p *Provider) getMovie(ctx context.Context, imdbID string, tmdbID int) (*titleprovider.Title, error) {
	var d movieDetails
	q := url.Values{}
	q.Set("append_to_response", "credits")
	if err := p.getJSON(ctx, "/movie/"+strconv.Itoa(tmdbID), q, &d); err != nil {
		return nil, err
	}

	t := &titleprovider.Title{
		ID:              imdbID,
		Type:            "movie",
		PrimaryTitle:    d.Title,
		PrimaryImage:    imageOf(d.PosterPath),
		StartYear:       yearFromDate(d.ReleaseDate),
		RuntimeSeconds:  d.Runtime * 60,
		Genres:          genreNames(d.Genres),
		Rating:          titleprovider.Rating{AggregateRating: d.VoteAverage, VoteCount: d.VoteCount},
		Plot:            d.Overview,
		Directors:       crewByJob(d.Credits.Crew, "Director"),
		Writers:         crewByDepartment(d.Credits.Crew, "Writing"),
		Stars:           castToPersons(d.Credits.Cast),
		OriginCountries: countryCodeNames(d.ProductionCountries),
		SpokenLanguages: languageCodeNames(d.SpokenLanguages),
	}
	return t, nil
}

func (p *Provider) getTV(ctx context.Context, imdbID string, tmdbID int) (*titleprovider.Title, error) {
	var d tvDetails
	q := url.Values{}
	q.Set("append_to_response", "credits,external_ids")
	if err := p.getJSON(ctx, "/tv/"+strconv.Itoa(tmdbID), q, &d); err != nil {
		return nil, err
	}

	runtime := 0
	if len(d.EpisodeRunTime) > 0 {
		runtime = d.EpisodeRunTime[0] * 60
	}

	t := &titleprovider.Title{
		ID:              imdbID,
		Type:            "tvSeries",
		PrimaryTitle:    d.Name,
		PrimaryImage:    imageOf(d.PosterPath),
		StartYear:       yearFromDate(d.FirstAirDate),
		RuntimeSeconds:  runtime,
		Genres:          genreNames(d.Genres),
		Rating:          titleprovider.Rating{AggregateRating: d.VoteAverage, VoteCount: d.VoteCount},
		Plot:            d.Overview,
		Directors:       crewByJob(d.Credits.Crew, "Director"),
		Writers:         crewByDepartment(d.Credits.Crew, "Writing"),
		Stars:           castToPersons(d.Credits.Cast),
		OriginCountries: countryCodeNames(d.ProductionCountries),
		SpokenLanguages: languageCodeNames(d.SpokenLanguages),
	}

	for _, s := range d.Seasons {
		if s.SeasonNumber < 1 { // skip Specials (season 0)
			continue
		}
		t.Seasons = append(t.Seasons, titleprovider.Season{
			Season:       strconv.Itoa(s.SeasonNumber),
			EpisodeCount: s.EpisodeCount,
		})

		var sd tmdbSeasonDetails
		if err := p.getJSON(ctx, fmt.Sprintf("/tv/%d/season/%d", tmdbID, s.SeasonNumber), nil, &sd); err != nil {
			return nil, err
		}
		for _, e := range sd.Episodes {
			t.Episodes = append(t.Episodes, mapEpisode(e))
		}
	}

	return t, nil
}

func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	var resp searchMultiResponse
	q := url.Values{}
	q.Set("query", query)
	if err := p.getJSON(ctx, "/search/multi", q, &resp); err != nil {
		return nil, err
	}

	items := make([]titleprovider.SearchItem, 0, limit)
	for _, r := range resp.Results {
		if len(items) >= limit {
			break
		}
		if r.MediaType != "movie" && r.MediaType != "tv" {
			continue
		}

		var ext tmdbExternalIDs
		path := fmt.Sprintf("/movie/%d/external_ids", r.ID)
		if r.MediaType == "tv" {
			path = fmt.Sprintf("/tv/%d/external_ids", r.ID)
		}
		if err := p.getJSON(ctx, path, nil, &ext); err != nil {
			return nil, err
		}
		if ext.IMDbID == "" { // drop titles with no linked IMDb ID
			continue
		}

		item := titleprovider.SearchItem{
			ID:           ext.IMDbID,
			PrimaryImage: imageOf(r.PosterPath),
			Rating:       titleprovider.Rating{AggregateRating: r.VoteAverage, VoteCount: r.VoteCount},
		}
		if r.MediaType == "movie" {
			item.Type = "movie"
			item.PrimaryTitle = r.Title
			item.StartYear = yearFromDate(r.ReleaseDate)
		} else {
			item.Type = "tvSeries"
			item.PrimaryTitle = r.Name
			item.StartYear = yearFromDate(r.FirstAirDate)
		}
		items = append(items, item)
	}
	return items, nil
}

// ----- mapping helpers -----

func imageOf(path string) titleprovider.Image {
	if path == "" {
		return titleprovider.Image{}
	}
	return titleprovider.Image{URL: imageBaseURL + path}
}

func yearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

func genreNames(gs []tmdbGenre) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

func countryCodeNames(cs []tmdbCountry) []titleprovider.CodeName {
	out := make([]titleprovider.CodeName, 0, len(cs))
	for _, c := range cs {
		out = append(out, titleprovider.CodeName{Code: c.ISO, Name: c.Name})
	}
	return out
}

func languageCodeNames(ls []tmdbLang) []titleprovider.CodeName {
	out := make([]titleprovider.CodeName, 0, len(ls))
	for _, l := range ls {
		name := l.Name
		if name == "" {
			name = l.ISO
		}
		out = append(out, titleprovider.CodeName{Code: l.ISO, Name: name})
	}
	return out
}

func castToPersons(cast []tmdbCast) []titleprovider.Person {
	out := make([]titleprovider.Person, 0, len(cast))
	for _, c := range cast {
		out = append(out, titleprovider.Person{
			ID:           strconv.Itoa(c.ID),
			DisplayName:  c.Name,
			PrimaryImage: profileImage(c.ProfilePath),
		})
	}
	return out
}

func crewByJob(crew []tmdbCrew, job string) []titleprovider.Person {
	out := []titleprovider.Person{}
	for _, c := range crew {
		if c.Job == job {
			out = append(out, titleprovider.Person{
				ID:           strconv.Itoa(c.ID),
				DisplayName:  c.Name,
				PrimaryImage: profileImage(c.ProfilePath),
			})
		}
	}
	return out
}

func crewByDepartment(crew []tmdbCrew, dept string) []titleprovider.Person {
	out := []titleprovider.Person{}
	seen := map[int]bool{}
	for _, c := range crew {
		if c.Department == dept && !seen[c.ID] {
			seen[c.ID] = true
			out = append(out, titleprovider.Person{
				ID:           strconv.Itoa(c.ID),
				DisplayName:  c.Name,
				PrimaryImage: profileImage(c.ProfilePath),
			})
		}
	}
	return out
}

func profileImage(path string) *titleprovider.Image {
	if path == "" {
		return nil
	}
	img := imageOf(path)
	return &img
}

func mapEpisode(e tmdbEpisode) titleprovider.Episode {
	ep := titleprovider.Episode{
		ID:            strconv.Itoa(e.ID),
		Title:         e.Name,
		PrimaryImage:  imageOf(e.StillPath),
		Season:        strconv.Itoa(e.SeasonNumber),
		EpisodeNumber: e.EpisodeNumber,
		Rating:        &titleprovider.Rating{AggregateRating: e.VoteAverage, VoteCount: e.VoteCount},
	}
	if e.Runtime != nil {
		secs := *e.Runtime * 60
		ep.RuntimeSeconds = &secs
	}
	if e.Overview != "" {
		plot := e.Overview
		ep.Plot = &plot
	}
	if rd := parseAirDate(e.AirDate); rd != nil {
		ep.ReleaseDate = rd
	}
	return ep
}

func parseAirDate(date string) *titleprovider.ReleaseDate {
	parts := strings.Split(date, "-")
	if len(parts) != 3 {
		return nil
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	return &titleprovider.ReleaseDate{Year: y, Month: m, Day: d}
}

// ----- TMDB wire types -----

type findResponse struct {
	MovieResults []findResult `json:"movie_results"`
	TVResults    []findResult `json:"tv_results"`
}
type findResult struct {
	ID int `json:"id"`
}

type movieDetails struct {
	ID                  int           `json:"id"`
	Title               string        `json:"title"`
	Overview            string        `json:"overview"`
	ReleaseDate         string        `json:"release_date"`
	Runtime             int           `json:"runtime"`
	PosterPath          string        `json:"poster_path"`
	VoteAverage         float64       `json:"vote_average"`
	VoteCount           int           `json:"vote_count"`
	Genres              []tmdbGenre   `json:"genres"`
	SpokenLanguages     []tmdbLang    `json:"spoken_languages"`
	ProductionCountries []tmdbCountry `json:"production_countries"`
	Credits             tmdbCredits   `json:"credits"`
	ExternalIDs         tmdbExternalIDs `json:"external_ids"`
}

type tvDetails struct {
	ID                  int                 `json:"id"`
	Name                string              `json:"name"`
	Overview            string              `json:"overview"`
	FirstAirDate        string              `json:"first_air_date"`
	EpisodeRunTime      []int               `json:"episode_run_time"`
	PosterPath          string              `json:"poster_path"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Genres              []tmdbGenre         `json:"genres"`
	SpokenLanguages     []tmdbLang          `json:"spoken_languages"`
	ProductionCountries []tmdbCountry       `json:"production_countries"`
	Credits             tmdbCredits         `json:"credits"`
	ExternalIDs         tmdbExternalIDs     `json:"external_ids"`
	Seasons             []tmdbSeasonSummary `json:"seasons"`
}

type tmdbSeasonSummary struct {
	SeasonNumber int `json:"season_number"`
	EpisodeCount int `json:"episode_count"`
}

type tmdbSeasonDetails struct {
	SeasonNumber int           `json:"season_number"`
	Episodes     []tmdbEpisode `json:"episodes"`
}

type tmdbEpisode struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	Runtime       *int    `json:"runtime"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

type tmdbGenre struct {
	Name string `json:"name"`
}
type tmdbLang struct {
	ISO  string `json:"iso_639_1"`
	Name string `json:"english_name"`
}
type tmdbCountry struct {
	ISO  string `json:"iso_3166_1"`
	Name string `json:"name"`
}
type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
	Crew []tmdbCrew `json:"crew"`
}
type tmdbCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}
type tmdbCrew struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}
type tmdbExternalIDs struct {
	IMDbID string `json:"imdb_id"`
}

type searchMultiResponse struct {
	Results []searchMultiResult `json:"results"`
}
type searchMultiResult struct {
	ID           int     `json:"id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	PosterPath   string  `json:"poster_path"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}
