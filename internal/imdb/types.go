// This files defines the struct that represents a movie/title document as found in ./sample_response.json,
// that is the response from the IMDB API (https://api.imdbapi.dev/titles/{titleID}).
// The ID is mapped to Mongo's _id via the bson tag so the same struct
// works for JSON (API) and MongoDB (storage).
package imdb

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
