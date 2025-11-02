package ratings

import "time"

type Rating struct {
	Id        string    `json:"id" bson:"_id"`
	TitleId   string    `json:"titleId" bson:"titleId"`
	UserId    string    `json:"userId" bson:"userId"`
	Note      float32   `json:"note" bson:"note"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type UpdateRatingRequest struct {
	Note float32 `json:"note" bson:"note"`
}

type AllRatingsFromTitle struct {
	Ratings []Rating `json:"ratings"`
}

type GetRatingsBatchRequest struct {
	Titles []string `json:"titles"`
}

// This will map titleId to respective ratings
type TitlesRatings struct {
	Titles map[string][]Rating `json:"titles"`
}
