package ratings

import "time"

type Rating struct {
	Id             string          `json:"id"`
	TitleId        string          `json:"titleId"`
	SeasonsRatings *SeasonsRatings `json:"seasonsRatings,omitempty"`
	UserId         string          `json:"userId"`
	Note           float32         `json:"note"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type SeasonRating struct {
	Rating    float32   `json:"rating"`
	AddedAt   time.Time `json:"addedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SeasonsRatings map[string]SeasonRating

type NewRating struct {
	GroupId string  `json:"groupId"`
	TitleId string  `json:"titleId"`
	Note    float32 `json:"note"`
	Season  *int    `json:"season,omitempty"`
}

type UpdateRatingRequest struct {
	Note   float32 `json:"note"`
	Season *int    `json:"season,omitempty"`
}

type AllRatingsFromTitle struct {
	Ratings []Rating `json:"ratings"`
}

type GetRatingsBatchRequest struct {
	Titles []string `json:"titles"`
}

type TitlesRatings struct {
	Titles map[string][]Rating `json:"titles"`
}
