package ratings

type Rating struct {
	Id       string  `json:"id" bson:"_id"`
	TitleId  string  `json:"titleId" bson:"titleId"`
	UserId   string  `json:"userId" bson:"userId"`
	Note     float32 `json:"note" bson:"note"`
	Comments string  `json:"comments" bson:"comments"`
}

type UpdateRatingRequest struct {
	Note     float32 `json:"note" bson:"note"`
	Comments string  `json:"comments" bson:"comments"`
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
