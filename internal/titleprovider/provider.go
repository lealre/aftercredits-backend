package titleprovider

import (
	"context"
	"errors"
)

// ErrTitleNotFound is returned when a title cannot be found for the given IMDb ID.
var ErrTitleNotFound = errors.New("title not found")

// Provider is the vendor-neutral interface every title metadata source implements.
type Provider interface {
	// GetTitle returns a fully-populated title by IMDb ID (tt...), including
	// seasons and episodes when it is a series. Implementations hide all
	// multi-call orchestration internally. Returns ErrTitleNotFound if absent.
	GetTitle(ctx context.Context, imdbID string) (*Title, error)

	// SearchTitles returns up to limit results, each identified by IMDb ID.
	SearchTitles(ctx context.Context, query string, limit int) ([]SearchItem, error)

	// Name returns the provider identifier, for logging.
	Name() string
}

// Title is the vendor-neutral representation of a movie or series.
type Title struct {
	ID              string
	Type            string // "movie" | "tvSeries" | "tvMiniSeries"
	PrimaryTitle    string
	PrimaryImage    Image
	StartYear       int
	RuntimeSeconds  int
	Genres          []string
	Rating          Rating
	Metacritic      *Metacritic // nil when the provider has no metacritic data
	Plot            string
	Directors       []Person
	Writers         []Person
	Stars           []Person
	OriginCountries []CodeName
	SpokenLanguages []CodeName
	Interests       []Interest // empty when the provider has no equivalent
	Seasons         []Season
	Episodes        []Episode
}

type Image struct {
	URL    string
	Width  int
	Height int
}

type Person struct {
	ID                 string
	DisplayName        string
	AlternativeNames   []string
	PrimaryImage       *Image
	PrimaryProfessions []string
}

type Rating struct {
	AggregateRating float64
	VoteCount       int
}

type Metacritic struct {
	Score       int
	ReviewCount int
}

type CodeName struct {
	Code string
	Name string
}

type Interest struct {
	ID         string
	Name       string
	IsSubgenre bool
}

type Season struct {
	Season       string
	EpisodeCount int
}

type Episode struct {
	ID             string
	Title          string
	PrimaryImage   Image
	Season         string
	EpisodeNumber  int
	RuntimeSeconds *int
	Plot           *string
	Rating         *Rating
	ReleaseDate    *ReleaseDate
}

type ReleaseDate struct {
	Year  int
	Month int
	Day   int
}

// SearchItem is a single search result, identified by IMDb ID.
type SearchItem struct {
	ID           string
	Type         string
	PrimaryTitle string
	PrimaryImage Image
	StartYear    int
	Rating       Rating
}
