package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ----- Types for the database -----

type CommentDb struct {
	Id        string    `json:"id" bson:"_id"`
	TitleId   string    `json:"titleId" bson:"titleId"`
	UserId    string    `json:"userId" bson:"userId"`
	Comment   string    `json:"comment" bson:"comment"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// ----- Methods for the database -----

func (db *DB) GetCommentsByTitleId(ctx context.Context, titleId string) ([]CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []CommentDb{}, err
	}
	defer cursor.Close(ctx)

	var comments []CommentDb
	if err = cursor.All(ctx, &comments); err != nil {
		return []CommentDb{}, err
	}

	if comments == nil {
		return []CommentDb{}, ErrRecordNotFound
	}

	return comments, nil
}

func (db *DB) AddComment(ctx context.Context, comment CommentDb) (CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	comment.Id = primitive.NewObjectID().Hex()

	now := time.Now()
	comment.CreatedAt = now
	comment.UpdatedAt = now

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
		return CommentDb{}, err
	}

	return comment, nil
}

func (db *DB) UpdateComment(ctx context.Context, commentDb CommentDb) error {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentDb.Id}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"comment":   commentDb.Comment,
			"updatedAt": now,
		},
	}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (db *DB) DeleteComment(ctx context.Context, commentId string) (int64, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentId}
	result, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}
