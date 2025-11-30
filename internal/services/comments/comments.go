package comments

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
)

func GetCommentsByTitleId(db *mongodb.DB, ctx context.Context, titleId, userId string) ([]Comment, error) {
	commentsDb, err := db.GetCommentsByTitleId(ctx, titleId, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return []Comment{}, nil
		}
		return []Comment{}, err
	}

	var comments []Comment
	for _, commentDb := range commentsDb {
		comments = append(comments, MapDbCommentToApiComment(commentDb))
	}

	return comments, nil
}

func AddComment(db *mongodb.DB, ctx context.Context, comment Comment) (Comment, error) {
	commentDb := mongodb.CommentDb{
		TitleId: comment.TitleId,
		UserId:  comment.UserId,
		Comment: comment.Comment,
	}

	commentDb, err := db.AddComment(ctx, commentDb)
	if err != nil {
		return Comment{}, err
	}

	return MapDbCommentToApiComment(commentDb), nil
}

func UpdateComment(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest) error {
	commentDb := mongodb.CommentDb{
		Id:      commentId,
		Comment: updateReq.Comment,
	}
	return db.UpdateComment(ctx, commentDb, userId)
}

func DeleteComment(db *mongodb.DB, ctx context.Context, commentId, userId string) (int64, error) {
	deletedCount, err := db.DeleteComment(ctx, commentId, userId)
	if err != nil {
		return 0, err
	}

	return deletedCount, nil
}

func MapDbCommentToApiComment(commentDb mongodb.CommentDb) Comment {
	return Comment(commentDb)
}
