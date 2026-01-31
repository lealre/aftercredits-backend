package comments

import "github.com/lealre/movies-backend/internal/mongodb"

func MapDbCommentToApiComment(commentDb mongodb.CommentDb) Comment {
	var seasonsComments *SeasonsComments
	if commentDb.SeasonsComments != nil {
		converted := make(SeasonsComments)
		for season, seasonCommentDb := range *commentDb.SeasonsComments {
			converted[season] = SeasonComment{
				Comment:   seasonCommentDb.Comment,
				AddedAt:   seasonCommentDb.AddedAt,
				UpdatedAt: seasonCommentDb.UpdatedAt,
			}
		}
		seasonsComments = &converted
	}

	return Comment{
		Id:              commentDb.Id,
		TitleId:         commentDb.TitleId,
		UserId:          commentDb.UserId,
		Comment:         commentDb.Comment,
		SeasonsComments: seasonsComments,
		CreatedAt:       commentDb.CreatedAt,
		UpdatedAt:       commentDb.UpdatedAt,
	}
}
