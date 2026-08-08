package models

import "time"

type SeasonRatingItem struct {
	Rating    float32
	AddedAt   time.Time
	UpdatedAt time.Time
}

type SeasonsRatings map[string]SeasonRatingItem

// UserRating is the storage-neutral representation of a user's rating of a
// title, carrying no persistence tags. Named
// UserRating rather than Rating because models.Rating is already taken by
// the title's embedded aggregate score (see title.go).
type UserRating struct {
	Id             string
	TitleId        string
	SeasonsRatings *SeasonsRatings
	UserId         string
	Note           float32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
