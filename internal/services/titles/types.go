// The structs defined here abstract some of the fields from the types in the imdb package
package titles

import (
	"encoding/json"
	"time"
)

type Title struct {
	ID              string     `json:"id"`
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

/*
FlexibleDate is a custom type that can unmarshal both date-only
and full datetime strings from JSON.

This type is used to facilitate updating the date when a title was watched,
as the specific hour, minutes, and seconds are typically not important
for tracking viewing history.
*/
type FlexibleDate struct {
	*time.Time
}

func (fd *FlexibleDate) UnmarshalJSON(data []byte) error {
	// Handle null values
	if string(data) == "null" {
		fd.Time = nil
		return nil
	}

	var dateStr string
	if err := json.Unmarshal(data, &dateStr); err != nil {
		return err
	}

	// Handle empty strings - set to nil
	if dateStr == "" {
		fd.Time = nil
		return nil
	}

	// Try different date formats
	formats := []string{
		"2006-01-02",                // YYYY-MM-DD
		"2006-01-02T15:04:05Z",      // ISO 8601 with Z
		"2006-01-02T15:04:05.000Z",  // ISO 8601 with milliseconds
		"2006-01-02T15:04:05-07:00", // ISO 8601 with timezone
		"2006-01-02 15:04:05",       // Space separated
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			fd.Time = &t
			return nil
		}
	}

	return &time.ParseError{
		Layout:     "multiple formats",
		Value:      dateStr,
		LayoutElem: "tried: 2006-01-02, 2006-01-02T15:04:05Z, 2006-01-02T15:04:05.000Z, 2006-01-02T15:04:05-07:00, 2006-01-02 15:04:05",
	}
}

func (fd *FlexibleDate) MarshalJSON() ([]byte, error) {
	if fd.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(fd.Time.Format("2006-01-02T15:04:05Z"))
}

type SetWatchedRequest struct {
	Watched   *bool         `json:"watched,omitempty"`
	WatchedAt *FlexibleDate `json:"watchedAt,omitempty"`
}
