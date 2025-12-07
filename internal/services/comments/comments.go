package comments

import (
	"context"
	"strings"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
* Gets all comments from a title in a specific group, including all users that are in the group.
* Assumes that the caller already checked that:
* - The group exists
* - The title is in the group
* - The user is in the group
 */
func GetCommentsByTitleId(db *mongodb.DB, ctx context.Context, groupId, titleId, userId string) ([]Comment, error) {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		return []Comment{}, err
	}

	commentsDb, err := db.GetCommentsByTitleId(ctx, titleId, group.Users)
	if err != nil {
		return []Comment{}, err
	}

	var comments []Comment
	for _, commentDb := range commentsDb {
		comments = append(comments, MapDbCommentToApiComment(commentDb))
	}

	return comments, nil
}

func AddComment(db *mongodb.DB, ctx context.Context, newComment NewComment, userId string) (Comment, error) {
	if strings.TrimSpace(newComment.Comment) == "" {
		return Comment{}, ErrCommentIsNull
	}

	commentDb := mongodb.CommentDb{
		TitleId: newComment.TitleId,
		UserId:  userId,
		Comment: newComment.Comment,
	}

	commentDb, err := db.AddComment(ctx, commentDb)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return Comment{}, ErrCommentAlreadyExists
		}
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
