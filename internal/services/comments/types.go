package comments

import "time"

type Comment struct {
	Id        string    `json:"id" bson:"_id"`
	TitleId   string    `json:"titleId" bson:"titleId"`
	UserId    string    `json:"userId" bson:"userId"`
	Comment   string    `json:"comment" bson:"comment"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type NewComment struct {
	TitleId string `json:"titleId" bson:"titleId"`
	GroupId string `json:"userId" bson:"userId"`
	Comment string `json:"comment" bson:"comment"`
}

type AllCommentsFromTitle struct {
	Comments []Comment `json:"comments"`
}

type UpdateCommentRequest struct {
	Comment string `json:"comment" bson:"comment"`
}
