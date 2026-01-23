package comments

import "github.com/lealre/movies-backend/internal/mongodb"

func MapDbCommentToApiComment(commentDb mongodb.CommentDb) Comment {
	var seasonsComments *SeasonsComments
	if commentDb.SeasonsComments != nil {
		converted := SeasonsComments(*commentDb.SeasonsComments)
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
