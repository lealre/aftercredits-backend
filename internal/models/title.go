package models

import (
	"slices"
	"time"
)

// seriesTitleTypes is the canonical spelling of the Title.Type values that
// denote a TV series (as opposed to "movie"). The pair is still written out
// inline in the services and the title providers; this is the one definition
// new code should reach for, and migrating the remaining sites onto it is
// separate work. It is an array, not a slice, so it cannot be reassigned or
// written through by a caller.
var seriesTitleTypes = [...]string{"tvSeries", "tvMiniSeries"}

// SeriesTitleTypes returns the Title.Type values that denote a TV series, as a
// fresh slice the caller may keep or pass to a store filter.
func SeriesTitleTypes() []string { return slices.Clone(seriesTitleTypes[:]) }

// IsSeriesTitleType reports whether a Title.Type denotes a TV series.
func IsSeriesTitleType(titleType string) bool {
	return slices.Contains(seriesTitleTypes[:], titleType)
}

// Title is the storage-neutral representation of a title (movie or TV
// series), carrying no persistence tags.
type Title struct {
	ID              string
	Type            string
	PrimaryTitle    string
	PrimaryImage    Image
	StartYear       int
	RuntimeSeconds  int
	Genres          []string
	Rating          Rating
	Metacritic      *Metacritic
	Plot            string
	Directors       []Person
	Writers         []Person
	Stars           []Person
	OriginCountries []CodeName
	SpokenLanguages []CodeName
	Interests       []Interest
	Seasons         []Seasons
	Episodes        []Episode
	AddedAt         *time.Time
	UpdatedAt       *time.Time
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

type Seasons struct {
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
