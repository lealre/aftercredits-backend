package models

import "time"

type SeasonCommentItem struct {
	Comment   string
	AddedAt   time.Time
	UpdatedAt time.Time
}

type SeasonsComments map[string]SeasonCommentItem

type Comment struct {
	Id              string
	TitleId         string
	UserId          string
	Comment         *string
	SeasonsComments *SeasonsComments
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
