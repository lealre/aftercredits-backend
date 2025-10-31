package comments

import "context"

func GetCommentsByTitleId(ctx context.Context, titleId string) ([]Comment, error) {
	return getCommentsByTitleIdDb(ctx, titleId)
}

func AddComment(ctx context.Context, comment Comment) (Comment, error) {
	return addCommentDb(ctx, comment)
}

func UpdateComment(ctx context.Context, commentId string, updateReq UpdateCommentRequest) error {
	return updateCommentDb(ctx, commentId, updateReq)
}

func DeleteComment(ctx context.Context, commentId string) (int64, error) {
	return deleteCommentDb(ctx, commentId)
}
