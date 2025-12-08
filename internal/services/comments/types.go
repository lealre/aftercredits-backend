package comments

import "time"

type Comment struct {
	Id        string    `json:"id"`
	TitleId   string    `json:"titleId"`
	UserId    string    `json:"userId"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NewComment struct {
	GroupId string `json:"groupId"`
	TitleId string `json:"titleId"`
	Comment string `json:"comment"`
}

type AllCommentsFromTitle struct {
	Comments []Comment `json:"comments"`
}

type UpdateCommentRequest struct {
	Comment string `json:"comment"`
}
