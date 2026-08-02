package comments

import "github.com/lealre/movies-backend/internal/models"

func MapDbCommentToApiComment(comment models.Comment) Comment {
	var seasonsComments *SeasonsComments
	if comment.SeasonsComments != nil {
		converted := make(SeasonsComments)
		for season, seasonComment := range *comment.SeasonsComments {
			converted[season] = SeasonComment{
				Comment:   seasonComment.Comment,
				AddedAt:   seasonComment.AddedAt,
				UpdatedAt: seasonComment.UpdatedAt,
			}
		}
		seasonsComments = &converted
	}

	return Comment{
		Id:              comment.Id,
		TitleId:         comment.TitleId,
		UserId:          comment.UserId,
		Comment:         comment.Comment,
		SeasonsComments: seasonsComments,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
	}
}
