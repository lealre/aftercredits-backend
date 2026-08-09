package models

import "time"

type SeasonCommentItem struct {
	Comment   string
	AddedAt   time.Time
	UpdatedAt time.Time
}

type SeasonsComments map[string]SeasonCommentItem

// Comment is a group-scoped fact: its identity is (UserId, TitleId, GroupId)
// and GroupId is always set — there is no comment outside a group.
type Comment struct {
	Id              string
	TitleId         string
	UserId          string
	GroupId         string
	Comment         *string
	SeasonsComments *SeasonsComments
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
