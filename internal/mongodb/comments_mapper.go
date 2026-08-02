package mongodb

import "github.com/lealre/movies-backend/internal/models"

// commentDbToModel converts the mongo-specific CommentDb into the
// storage-neutral models.Comment used by the service layer.
func commentDbToModel(c CommentDb) models.Comment {
	return models.Comment{
		Id:              c.Id,
		TitleId:         c.TitleId,
		UserId:          c.UserId,
		Comment:         c.Comment,
		SeasonsComments: seasonsCommentsDbToModel(c.SeasonsComments),
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// commentModelToDb converts a storage-neutral models.Comment back into the
// mongo-specific CommentDb used at the persistence boundary.
func commentModelToDb(c models.Comment) CommentDb {
	return CommentDb{
		Id:              c.Id,
		TitleId:         c.TitleId,
		UserId:          c.UserId,
		Comment:         c.Comment,
		SeasonsComments: seasonsCommentsModelToDb(c.SeasonsComments),
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// ----- SeasonsComments -----

func seasonCommentItemDbToModel(s SeasonCommentItemDb) models.SeasonCommentItem {
	return models.SeasonCommentItem{
		Comment:   s.Comment,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonCommentItemModelToDb(s models.SeasonCommentItem) SeasonCommentItemDb {
	return SeasonCommentItemDb{
		Comment:   s.Comment,
		AddedAt:   s.AddedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func seasonsCommentsDbToModel(s *SeasonsCommentsDb) *models.SeasonsComments {
	if s == nil {
		return nil
	}
	out := make(models.SeasonsComments, len(*s))
	for k, v := range *s {
		out[k] = seasonCommentItemDbToModel(v)
	}
	return &out
}

func seasonsCommentsModelToDb(s *models.SeasonsComments) *SeasonsCommentsDb {
	if s == nil {
		return nil
	}
	out := make(SeasonsCommentsDb, len(*s))
	for k, v := range *s {
		out[k] = seasonCommentItemModelToDb(v)
	}
	return &out
}
