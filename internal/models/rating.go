package models

import "time"

type SeasonRatingItem struct {
	Rating    float64
	AddedAt   time.Time
	UpdatedAt time.Time
}

type SeasonsRatings map[string]SeasonRatingItem

// UserRating is the storage-neutral representation of a user's rating of a
// title, carrying no persistence tags. Named
// UserRating rather than Rating because models.Rating is already taken by
// the title's embedded aggregate score (see title.go).
//
// A rating is a group-scoped fact: its identity is (UserId, TitleId, GroupId)
// and GroupId is always set — there is no rating outside a group.
type UserRating struct {
	Id             string
	TitleId        string
	SeasonsRatings *SeasonsRatings
	UserId         string
	GroupId        string
	Note           float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
