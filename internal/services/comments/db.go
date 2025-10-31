package comments

import (
	"context"
	"time"

	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func getCommentsByTitleIdDb(ctx context.Context, titleId string) ([]Comment, error) {
	coll := mongodb.GetCommentsCollection(ctx)

	filter := bson.M{"titleId": titleId}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var comments []Comment
	if err = cursor.All(ctx, &comments); err != nil {
		return nil, err
	}

	return comments, nil
}

func addCommentDb(ctx context.Context, comment Comment) (Comment, error) {
	coll := mongodb.GetCommentsCollection(ctx)

	comment.Id = primitive.NewObjectID().Hex()

	now := time.Now()
	comment.CreatedAt = &now
	comment.UpdatedAt = &now

	doc := map[string]any{
		"_id":       comment.Id,
		"titleId":   comment.TitleId,
		"userId":    comment.UserId,
		"comment":   comment.Comment,
		"createdAt": comment.CreatedAt,
		"updatedAt": comment.UpdatedAt,
	}

	_, err := coll.InsertOne(ctx, doc)
	if err != nil {
		return Comment{}, err
	}

	return comment, nil
}

func updateCommentDb(ctx context.Context, commentId string, updateReq UpdateCommentRequest) error {
	coll := mongodb.GetCommentsCollection(ctx)

	filter := bson.M{"_id": commentId}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"comment":   updateReq.Comment,
			"updatedAt": now,
		},
	}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return mongodb.ErrRecordNotFound
	}

	return nil
}

func deleteCommentDb(ctx context.Context, commentId string) (int64, error) {
	coll := mongodb.GetCommentsCollection(ctx)

	filter := bson.M{"_id": commentId}
	result, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}
