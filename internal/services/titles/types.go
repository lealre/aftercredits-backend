// The structs defined here abstract some of the fields from the types in the imdb package
package titles

// Movie represents a movie with all its details
type Movie struct {
	ID              string   `json:"id"`
	PrimaryTitle    string   `json:"primaryTitle"`
	PrimaryImage    Image    `json:"primaryImage"`
	StartYear       int      `json:"startYear"`
	RuntimeSeconds  int      `json:"runtimeSeconds"`
	Genres          []string `json:"genres"`
	Rating          Rating   `json:"rating"`
	Plot            string   `json:"plot"`
	DirectorsNames  []string `json:"directorsNames"`
	WritersNames    []string `json:"writersNames"`
	StarsNames      []string `json:"starsNames"`
	OriginCountries []string `json:"originCountries"`
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

type AddMovieRequest struct {
	URL string `json:"url"`
}

type AllMoviesResponse struct {
	Movies []Movie `json:"movies"`
}
