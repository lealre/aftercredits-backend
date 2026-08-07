package models

import "time"

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
