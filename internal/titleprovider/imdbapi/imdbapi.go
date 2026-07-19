package imdbapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

const defaultBaseURL = "https://api.imdbapi.dev"

// Provider implements titleprovider.Provider against api.imdbapi.dev.
type Provider struct {
	baseURL string
	client  *http.Client
}

// New returns an imdbapi.dev provider using the public base URL.
func New() *Provider {
	return &Provider{baseURL: defaultBaseURL, client: http.DefaultClient}
}

func newWithBaseURL(baseURL string) *Provider {
	return &Provider{baseURL: baseURL, client: http.DefaultClient}
}

func (p *Provider) Name() string { return "imdbapi" }

func (p *Provider) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := p.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
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
		return fmt.Errorf("imdbapi: non-2xx status %s for %s - %s", resp.Status, path, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	var wt wireTitle
	if err := p.getJSON(ctx, "/titles/"+imdbID, nil, &wt); err != nil {
		return nil, err
	}
	t := mapTitle(wt)

	if wt.Type == "tvSeries" || wt.Type == "tvMiniSeries" {
		var sr wireSeasonsResponse
		if err := p.getJSON(ctx, "/titles/"+imdbID+"/seasons", nil, &sr); err != nil {
			return nil, err
		}
		for _, s := range sr.Seasons {
			t.Seasons = append(t.Seasons, titleprovider.Season{Season: s.Season, EpisodeCount: s.EpisodeCount})
		}

		pageToken := ""
		for {
			q := url.Values{}
			q.Set("pageSize", "50")
			if pageToken != "" {
				q.Set("pageToken", pageToken)
			}
			var er wireEpisodesResponse
			if err := p.getJSON(ctx, "/titles/"+imdbID+"/episodes", q, &er); err != nil {
				return nil, err
			}
			for _, e := range er.Episodes {
				t.Episodes = append(t.Episodes, mapEpisode(e))
			}
			if er.NextPageToken == "" {
				break
			}
			pageToken = er.NextPageToken
		}
	}

	return &t, nil
}

func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))

	var sr wireSearchResponse
	if err := p.getJSON(ctx, "/search/titles", q, &sr); err != nil {
		return nil, err
	}

	items := make([]titleprovider.SearchItem, 0, len(sr.Titles))
	for _, it := range sr.Titles {
		items = append(items, titleprovider.SearchItem{
			ID:           it.ID,
			Type:         it.Type,
			PrimaryTitle: it.PrimaryTitle,
			PrimaryImage: titleprovider.Image{URL: it.PrimaryImage.URL, Width: it.PrimaryImage.Width, Height: it.PrimaryImage.Height},
			StartYear:    it.StartYear,
			Rating:       titleprovider.Rating{AggregateRating: it.Rating.AggregateRating, VoteCount: it.Rating.VoteCount},
		})
	}
	return items, nil
}

// ----- mapping -----

func mapTitle(wt wireTitle) titleprovider.Title {
	t := titleprovider.Title{
		ID:             wt.ID,
		Type:           wt.Type,
		PrimaryTitle:   wt.PrimaryTitle,
		PrimaryImage:   mapImage(wt.PrimaryImage),
		StartYear:      wt.StartYear,
		RuntimeSeconds: wt.RuntimeSeconds,
		Genres:         wt.Genres,
		Rating:         titleprovider.Rating{AggregateRating: wt.Rating.AggregateRating, VoteCount: wt.Rating.VoteCount},
		Plot:           wt.Plot,
		Directors:      mapPersons(wt.Directors),
		Writers:        mapPersons(wt.Writers),
		Stars:          mapPersons(wt.Stars),
	}
	if wt.Metacritic != nil {
		t.Metacritic = &titleprovider.Metacritic{Score: wt.Metacritic.Score, ReviewCount: wt.Metacritic.ReviewCount}
	}
	for _, c := range wt.OriginCountries {
		t.OriginCountries = append(t.OriginCountries, titleprovider.CodeName{Code: c.Code, Name: c.Name})
	}
	for _, l := range wt.SpokenLanguages {
		t.SpokenLanguages = append(t.SpokenLanguages, titleprovider.CodeName{Code: l.Code, Name: l.Name})
	}
	for _, i := range wt.Interests {
		t.Interests = append(t.Interests, titleprovider.Interest{ID: i.ID, Name: i.Name, IsSubgenre: i.IsSubgenre})
	}
	return t
}

func mapImage(i wireImage) titleprovider.Image {
	return titleprovider.Image{URL: i.URL, Width: i.Width, Height: i.Height}
}

func mapPersons(ps []wirePerson) []titleprovider.Person {
	out := make([]titleprovider.Person, 0, len(ps))
	for _, p := range ps {
		person := titleprovider.Person{
			ID:                 p.ID,
			DisplayName:        p.DisplayName,
			AlternativeNames:   p.AlternativeNames,
			PrimaryProfessions: p.PrimaryProfessions,
		}
		if p.PrimaryImage != nil {
			img := mapImage(*p.PrimaryImage)
			person.PrimaryImage = &img
		}
		out = append(out, person)
	}
	return out
}

func mapEpisode(e wireEpisode) titleprovider.Episode {
	ep := titleprovider.Episode{
		ID:             e.ID,
		Title:          e.Title,
		PrimaryImage:   mapImage(e.PrimaryImage),
		Season:         e.Season,
		EpisodeNumber:  e.EpisodeNumber,
		RuntimeSeconds: e.RuntimeSeconds,
		Plot:           e.Plot,
	}
	if e.Rating != nil {
		ep.Rating = &titleprovider.Rating{AggregateRating: e.Rating.AggregateRating, VoteCount: e.Rating.VoteCount}
	}
	if e.ReleaseDate != nil {
		ep.ReleaseDate = &titleprovider.ReleaseDate{Year: e.ReleaseDate.Year, Month: e.ReleaseDate.Month, Day: e.ReleaseDate.Day}
	}
	return ep
}

// ----- imdbapi.dev wire types -----

type wireTitle struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	PrimaryTitle    string          `json:"primaryTitle"`
	PrimaryImage    wireImage       `json:"primaryImage"`
	StartYear       int             `json:"startYear"`
	RuntimeSeconds  int             `json:"runtimeSeconds"`
	Genres          []string        `json:"genres"`
	Rating          wireRating      `json:"rating"`
	Metacritic      *wireMetacritic `json:"metacritic,omitempty"`
	Plot            string          `json:"plot"`
	Directors       []wirePerson    `json:"directors"`
	Writers         []wirePerson    `json:"writers"`
	Stars           []wirePerson    `json:"stars"`
	OriginCountries []wireCodeName  `json:"originCountries"`
	SpokenLanguages []wireCodeName  `json:"spokenLanguages"`
	Interests       []wireInterest  `json:"interests"`
}

type wireImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type wirePerson struct {
	ID                 string     `json:"id"`
	DisplayName        string     `json:"displayName"`
	AlternativeNames   []string   `json:"alternativeNames,omitempty"`
	PrimaryImage       *wireImage `json:"primaryImage,omitempty"`
	PrimaryProfessions []string   `json:"primaryProfessions,omitempty"`
}
type wireRating struct {
	AggregateRating float64 `json:"aggregateRating"`
	VoteCount       int     `json:"voteCount"`
}
type wireMetacritic struct {
	Score       int `json:"score"`
	ReviewCount int `json:"reviewCount"`
}
type wireCodeName struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type wireInterest struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsSubgenre bool   `json:"isSubgenre,omitempty"`
}
type wireSeasonsResponse struct {
	Seasons []wireSeason `json:"seasons"`
}
type wireSeason struct {
	Season       string `json:"season"`
	EpisodeCount int    `json:"episodeCount"`
}
type wireEpisodesResponse struct {
	Episodes      []wireEpisode `json:"episodes"`
	TotalCount    int           `json:"totalCount"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}
type wireEpisode struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	PrimaryImage   wireImage        `json:"primaryImage"`
	Season         string           `json:"season"`
	EpisodeNumber  int              `json:"episodeNumber"`
	RuntimeSeconds *int             `json:"runtimeSeconds,omitempty"`
	Plot           *string          `json:"plot,omitempty"`
	Rating         *wireRating      `json:"rating,omitempty"`
	ReleaseDate    *wireReleaseDate `json:"releaseDate,omitempty"`
}
type wireReleaseDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}
type wireSearchResponse struct {
	Titles []wireSearchItem `json:"titles"`
}
type wireSearchItem struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	PrimaryTitle string     `json:"primaryTitle"`
	PrimaryImage wireImage  `json:"primaryImage"`
	StartYear    int        `json:"startYear"`
	Rating       wireRating `json:"rating"`
}
