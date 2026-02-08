// This files defines the struct that represents a title document
// that is the response from the IMDb API (https://api.imdbapi.dev/titles/{titleID}).
// The ID is mapped to Mongo's _id via the bson tag so the same struct
// works for JSON (API) and MongoDB (storage).
package imdb

import (
	"time"
)

type Title struct {
	ID              string      `json:"id" bson:"_id"`
	Type            string      `json:"type" bson:"type"`
	PrimaryTitle    string      `json:"primaryTitle" bson:"primaryTitle"`
	PrimaryImage    Image       `json:"primaryImage" bson:"primaryImage"`
	StartYear       int         `json:"startYear" bson:"startYear"`
	RuntimeSeconds  int         `json:"runtimeSeconds" bson:"runtimeSeconds"`
	Genres          []string    `json:"genres" bson:"genres"`
	Rating          Rating      `json:"rating" bson:"rating"`
	Metacritic      *Metacritic `json:"metacritic,omitempty" bson:"metacritic,omitempty"`
	Plot            string      `json:"plot" bson:"plot"`
	Directors       []Person    `json:"directors" bson:"directors"`
	Writers         []Person    `json:"writers" bson:"writers"`
	Stars           []Person    `json:"stars" bson:"stars"`
	OriginCountries []CodeName  `json:"originCountries" bson:"originCountries"`
	SpokenLanguages []CodeName  `json:"spokenLanguages" bson:"spokenLanguages"`
	Interests       []Interest  `json:"interests" bson:"interests"`
	Seasons         *[]Seasons  `json:"seasons,omitempty"`
	Episodes        *[]Episode  `json:"episodes,omitempty"`
	AddedAt         *time.Time  `json:"addedAt,omitempty" bson:"addedAt,omitempty"`
	UpdatedAt       *time.Time  `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

type Image struct {
	URL    string `json:"url" bson:"url"`
	Width  int    `json:"width" bson:"width"`
	Height int    `json:"height" bson:"height"`
}

type Person struct {
	ID                 string   `json:"id" bson:"id"`
	DisplayName        string   `json:"displayName" bson:"displayName"`
	AlternativeNames   []string `json:"alternativeNames,omitempty" bson:"alternativeNames,omitempty"`
	PrimaryImage       *Image   `json:"primaryImage,omitempty" bson:"primaryImage,omitempty"`
	PrimaryProfessions []string `json:"primaryProfessions,omitempty" bson:"primaryProfessions,omitempty"`
}

type Rating struct {
	AggregateRating float64 `json:"aggregateRating" bson:"aggregateRating"`
	VoteCount       int     `json:"voteCount" bson:"voteCount"`
}

type Metacritic struct {
	Score       int `json:"score" bson:"score"`
	ReviewCount int `json:"reviewCount" bson:"reviewCount"`
}

type CodeName struct {
	Code string `json:"code" bson:"code"`
	Name string `json:"name" bson:"name"`
}

type Interest struct {
	ID         string `json:"id" bson:"id"`
	Name       string `json:"name" bson:"name"`
	IsSubgenre bool   `json:"isSubgenre,omitempty" bson:"isSubgenre,omitempty"`
}

type EpisodesResponse struct {
	Episodes      []Episode `json:"episodes" bson:"episodes"`
	TotalCount    int       `json:"totalCount" bson:"totalCount"`
	NextPageToken string    `json:"nextPageToken,omitempty" bson:"nextPageToken,omitempty"`
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

type SeasonsResponse struct {
	Seasons []Seasons `json:"seasons" bson:"seasons"`
}

type Seasons struct {
	Season       string `json:"season" bson:"season"`
	EpisodeCount int    `json:"episodeCount" bson:"episodeCount"`
}

type BatchTitlesResponse struct {
	Titles []Title `json:"titles" bson:"titles"`
}
