// Package omdb implements titleprovider.Provider against the OMDb API
// (https://www.omdbapi.com). OMDb is keyed directly by IMDb ID and returns the
// real IMDb rating (imdbRating) and vote count, plus a Metacritic score — data
// TMDB does not expose. See ../README.md for a provider comparison.
package omdb

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

const defaultBaseURL = "https://www.omdbapi.com"

// Provider implements titleprovider.Provider against OMDb.
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New returns an OMDb provider. apiKey is a free OMDb API key (omdbapi.com/apikey.aspx).
func New(apiKey string) *Provider {
	return &Provider{baseURL: defaultBaseURL, apiKey: apiKey, client: http.DefaultClient}
}

func newWithBaseURL(baseURL, apiKey string) *Provider {
	return &Provider{baseURL: baseURL, apiKey: apiKey, client: http.DefaultClient}
}

func (p *Provider) Name() string { return "omdb" }

// get issues a GET against OMDb with the api key attached and decodes the JSON
// body into out. OMDb always replies 200; logical errors live in the body's
// Response/Error fields, which callers inspect.
func (p *Provider) get(ctx context.Context, params url.Values, out any) error {
	params.Set("apikey", p.apiKey)
	reqURL := p.baseURL + "/?" + params.Encode()

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
		return fmt.Errorf("omdb: non-2xx status %s - %s", resp.Status, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// omdbErr classifies an OMDb "Response":"False" body: genuine not-found becomes
// ErrTitleNotFound; anything else (e.g. "Request limit reached!") is a real error.
func omdbErr(errMsg string) error {
	low := strings.ToLower(errMsg)
	if strings.Contains(low, "not found") || strings.Contains(low, "incorrect imdb id") {
		return titleprovider.ErrTitleNotFound
	}
	return fmt.Errorf("omdb: %s", errMsg)
}

func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	var wt wireTitle
	q := url.Values{}
	q.Set("i", imdbID)
	q.Set("plot", "full")
	if err := p.get(ctx, q, &wt); err != nil {
		return nil, err
	}
	if wt.Response != "True" {
		return nil, omdbErr(wt.Error)
	}

	t := mapTitle(wt)

	if mapType(wt.Type) == "tvSeries" || mapType(wt.Type) == "tvMiniSeries" {
		total, _ := strconv.Atoi(strings.TrimSpace(wt.TotalSeasons))
		for n := 1; n <= total; n++ {
			var ws wireSeason
			sq := url.Values{}
			sq.Set("i", imdbID)
			sq.Set("Season", strconv.Itoa(n))
			if err := p.get(ctx, sq, &ws); err != nil {
				return nil, err
			}
			if ws.Response != "True" {
				continue // skip a season OMDb can't return, rather than failing the whole title
			}
			t.Seasons = append(t.Seasons, titleprovider.Season{
				Season:       strconv.Itoa(n),
				EpisodeCount: len(ws.Episodes),
			})
			for _, e := range ws.Episodes {
				t.Episodes = append(t.Episodes, mapEpisode(e, n))
			}
		}
	}
	return &t, nil
}

// RatingByID fetches only the IMDb rating (and Metacritic, if present) for a
// title, in a single request and without fetching seasons. The hybrid provider
// uses this to overlay the IMDb rating onto another provider's metadata.
func (p *Provider) RatingByID(ctx context.Context, imdbID string) (titleprovider.Rating, *titleprovider.Metacritic, error) {
	var wt wireTitle
	q := url.Values{}
	q.Set("i", imdbID)
	if err := p.get(ctx, q, &wt); err != nil {
		return titleprovider.Rating{}, nil, err
	}
	if wt.Response != "True" {
		return titleprovider.Rating{}, nil, omdbErr(wt.Error)
	}
	return titleprovider.Rating{
		AggregateRating: parseFloat(wt.ImdbRating),
		VoteCount:       parseVotes(wt.ImdbVotes),
	}, parseMetacritic(wt.Metascore), nil
}

func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	var ws wireSearch
	q := url.Values{}
	q.Set("s", query)
	if err := p.get(ctx, q, &ws); err != nil {
		return nil, err
	}
	if ws.Response != "True" {
		// "Movie not found!" on search just means no results.
		if strings.Contains(strings.ToLower(ws.Error), "not found") {
			return []titleprovider.SearchItem{}, nil
		}
		return nil, omdbErr(ws.Error)
	}

	items := make([]titleprovider.SearchItem, 0, limit)
	for _, it := range ws.Search {
		if len(items) >= limit {
			break
		}
		items = append(items, titleprovider.SearchItem{
			ID:           it.ImdbID,
			Type:         mapType(it.Type),
			PrimaryTitle: it.Title,
			PrimaryImage: imageOf(it.Poster),
			StartYear:    parseYear(it.Year),
		})
	}
	return items, nil
}

// ----- mapping -----

func mapTitle(wt wireTitle) titleprovider.Title {
	t := titleprovider.Title{
		ID:             wt.ImdbID,
		Type:           mapType(wt.Type),
		PrimaryTitle:   wt.Title,
		PrimaryImage:   imageOf(wt.Poster),
		StartYear:      parseYear(wt.Year),
		RuntimeSeconds: parseRuntimeSeconds(wt.Runtime),
		Genres:         parseList(wt.Genre),
		Rating:         titleprovider.Rating{AggregateRating: parseFloat(wt.ImdbRating), VoteCount: parseVotes(wt.ImdbVotes)},
		Metacritic:     parseMetacritic(wt.Metascore),
		Plot:           naOrEmpty(wt.Plot),
		Directors:      parsePersons(wt.Director),
		Writers:        parsePersons(wt.Writer),
		Stars:          parsePersons(wt.Actors),
	}
	if c := naOrEmpty(wt.Country); c != "" {
		for _, name := range parseList(wt.Country) {
			t.OriginCountries = append(t.OriginCountries, titleprovider.CodeName{Name: name})
		}
	}
	if l := naOrEmpty(wt.Language); l != "" {
		for _, name := range parseList(wt.Language) {
			t.SpokenLanguages = append(t.SpokenLanguages, titleprovider.CodeName{Name: name})
		}
	}
	return t
}

func mapEpisode(e wireEpisode, seasonNum int) titleprovider.Episode {
	ep := titleprovider.Episode{
		ID:            e.ImdbID,
		Title:         e.Title,
		Season:        strconv.Itoa(seasonNum),
		EpisodeNumber: atoiSafe(e.Episode),
	}
	if r := parseFloat(e.ImdbRating); r > 0 {
		ep.Rating = &titleprovider.Rating{AggregateRating: r}
	}
	if rd := parseISODate(e.Released); rd != nil {
		ep.ReleaseDate = rd
	}
	return ep
}

func mapType(t string) string {
	switch strings.ToLower(t) {
	case "movie":
		return "movie"
	case "series":
		return "tvSeries"
	default:
		return t
	}
}

func naOrEmpty(s string) string {
	if s == "" || s == "N/A" {
		return ""
	}
	return s
}

func imageOf(poster string) titleprovider.Image {
	if p := naOrEmpty(poster); p != "" {
		return titleprovider.Image{URL: p}
	}
	return titleprovider.Image{}
}

func parseYear(s string) int {
	s = naOrEmpty(s)
	if len(s) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(s[:4])
	return y
}

func parseRuntimeSeconds(s string) int {
	s = naOrEmpty(s)
	fields := strings.Fields(s) // "127 min" -> ["127","min"]
	if len(fields) == 0 {
		return 0
	}
	mins, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0
	}
	return mins * 60
}

func parseList(s string) []string {
	if naOrEmpty(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parsePersons(s string) []titleprovider.Person {
	names := parseList(s)
	out := make([]titleprovider.Person, 0, len(names))
	for _, n := range names {
		out = append(out, titleprovider.Person{DisplayName: n})
	}
	return out
}

func parseFloat(s string) float64 {
	if naOrEmpty(s) == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func parseVotes(s string) int {
	s = naOrEmpty(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseMetacritic(metascore string) *titleprovider.Metacritic {
	s := naOrEmpty(metascore)
	if s == "" {
		return nil
	}
	score, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &titleprovider.Metacritic{Score: score}
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// parseISODate parses an OMDb episode "Released" value like "2008-01-20".
func parseISODate(s string) *titleprovider.ReleaseDate {
	if naOrEmpty(s) == "" {
		return nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return nil
	}
	y, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	d, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil {
		return nil
	}
	return &titleprovider.ReleaseDate{Year: y, Month: m, Day: d}
}

// ----- OMDb wire types -----

type wireTitle struct {
	Title        string `json:"Title"`
	Year         string `json:"Year"`
	Released     string `json:"Released"`
	Runtime      string `json:"Runtime"`
	Genre        string `json:"Genre"`
	Director     string `json:"Director"`
	Writer       string `json:"Writer"`
	Actors       string `json:"Actors"`
	Plot         string `json:"Plot"`
	Country      string `json:"Country"`
	Language     string `json:"Language"`
	Poster       string `json:"Poster"`
	Metascore    string `json:"Metascore"`
	ImdbRating   string `json:"imdbRating"`
	ImdbVotes    string `json:"imdbVotes"`
	ImdbID       string `json:"imdbID"`
	Type         string `json:"Type"`
	TotalSeasons string `json:"totalSeasons"`
	Response     string `json:"Response"`
	Error        string `json:"Error"`
}

type wireSeason struct {
	Episodes []wireEpisode `json:"Episodes"`
	Response string        `json:"Response"`
	Error    string        `json:"Error"`
}

type wireEpisode struct {
	Title      string `json:"Title"`
	Released   string `json:"Released"`
	Episode    string `json:"Episode"`
	ImdbRating string `json:"imdbRating"`
	ImdbID     string `json:"imdbID"`
}

type wireSearch struct {
	Search   []wireSearchItem `json:"Search"`
	Response string           `json:"Response"`
	Error    string           `json:"Error"`
}

type wireSearchItem struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Poster string `json:"Poster"`
}
