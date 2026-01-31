// The structs defined here abstract some of the fields from the types in the imdb package
package titles

import (
	"time"

	"github.com/lealre/movies-backend/internal/generics"
)

type Title struct {
	Id              string     `json:"id"`
	PrimaryTitle    string     `json:"primaryTitle"`
	Type            string     `json:"type"`
	PrimaryImage    Image      `json:"primaryImage"`
	StartYear       int        `json:"startYear"`
	RuntimeSeconds  int        `json:"runtimeSeconds"`
	Genres          []string   `json:"genres"`
	Rating          Rating     `json:"rating"`
	Plot            string     `json:"plot"`
	DirectorsNames  []string   `json:"directorsNames"`
	WritersNames    []string   `json:"writersNames"`
	StarsNames      []string   `json:"starsNames"`
	Seasons         []Seasons  `json:"seasons"`
	Episodes        []Episode  `json:"episodes"`
	OriginCountries []string   `json:"originCountries"`
	AddedAt         *time.Time `json:"addedAt,omitempty"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
}

type TitleResponse struct {
	Id              string     `json:"id"`
	PrimaryTitle    string     `json:"primaryTitle"`
	Type            string     `json:"type"`
	PrimaryImage    Image      `json:"primaryImage"`
	StartYear       int        `json:"startYear"`
	RuntimeSeconds  int        `json:"runtimeSeconds"`
	Genres          []string   `json:"genres"`
	Rating          Rating     `json:"rating"`
	Plot            string     `json:"plot"`
	DirectorsNames  []string   `json:"directorsNames"`
	WritersNames    []string   `json:"writersNames"`
	StarsNames      []string   `json:"starsNames"`
	OriginCountries []string   `json:"originCountries"`
	Watched         bool       `json:"watched"`
	AddedAt         *time.Time `json:"addedAt,omitempty"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
	WatchedAt       *time.Time `json:"watchedAt,omitempty"`
}

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Rating struct {
	AggregateRating float64 `json:"aggregateRating"`
	VoteCount       int     `json:"voteCount"`
}

type AddTitleRequest struct {
	URL string `json:"url"`
}

type AllTitlesResponse struct {
	Titles []Title `json:"titles"`
}

type SetWatchedRequest struct {
	Watched   *bool                  `json:"watched,omitempty"`
	WatchedAt *generics.FlexibleDate `json:"watchedAt,omitempty"`
}

type Seasons struct {
	Season       string `json:"season"`
	EpisodeCount int    `json:"episodeCount"`
}

type Episode struct {
	ID             string       `json:"id" bson:"id"`
	Title          string       `json:"title" bson:"title"`
	PrimaryImage   Image        `json:"primaryImage" bson:"primaryImage"`
	Season         string       `json:"season" bson:"season"`
	EpisodeNumber  int          `json:"episodeNumber" bson:"episodeNumber"`
	RuntimeSeconds *int         `json:"runtimeSeconds,omitempty" bson:"runtimeSeconds,omitempty"`
	Plot           *string      `json:"plot,omitempty" bson:"plot,omitempty"`
	Rating         *Rating      `json:"rating,omitempty" bson:"rating,omitempty"`
	ReleaseDate    *ReleaseDate `json:"releaseDate,omitempty" bson:"releaseDate,omitempty"`
}

type ReleaseDate struct {
	Year  int `json:"year" bson:"year"`
	Month int `json:"month" bson:"month"`
	Day   int `json:"day" bson:"day"`
}
