package ratings

type Rating struct {
	Id       string `json:"id" bson:"_id"`
	TitleId  string `json:"titleId" bson:"titleId"`
	UserId   string `json:"userId" bson:"userId"`
	Note     int    `json:"note" bson:"note"`
	Comments string `json:"comments" bson:"comments"`
}

type AllRatingsFromMovie struct {
	Ratings []Rating `json:"ratings"`
}
