package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ----- Types for the database -----

type CommentDb struct {
	Id              string             `json:"id" bson:"_id"`
	TitleId         string             `json:"titleId" bson:"titleId"`
	UserId          string             `json:"userId" bson:"userId"`
	Comment         *string            `json:"comment" bson:"comment"`
	SeasonsComments *SeasonsCommentsDb `json:"seasonsComments" bson:"seasonsComments"`
	CreatedAt       time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type SeasonCommentItemDb struct {
	Comment   string    `json:"comment" bson:"comment"`
	AddedAt   time.Time `json:"addedAt" bson:"addedAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type SeasonsCommentsDb map[string]SeasonCommentItemDb

// ----- Methods for the database -----

func (db *DB) GetCommentsByTitleId(ctx context.Context, titleId string, usersFromGroup []string) ([]CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId, "userId": bson.M{"$in": usersFromGroup}}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []CommentDb{}, err
	}
	defer cursor.Close(ctx)

	var comments []CommentDb
	if err = cursor.All(ctx, &comments); err != nil {
		return []CommentDb{}, err
	}

	return comments, nil
}

func (db *DB) GetUserCommentByTitleId(ctx context.Context, titleId string, userId string) (CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId, "userId": userId}

	var comment CommentDb
	err := coll.FindOne(ctx, filter).Decode(&comment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return CommentDb{}, ErrRecordNotFound
		}
		return CommentDb{}, err
	}

	return comment, nil
}

func (db *DB) GetCommentById(ctx context.Context, commentId string, userId string) (CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentId, "userId": userId}

	var comment CommentDb
	err := coll.FindOne(ctx, filter).Decode(&comment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return CommentDb{}, ErrRecordNotFound
		}
		return CommentDb{}, err
	}

	return comment, nil
}

func (db *DB) AddComment(ctx context.Context, comment CommentDb) (CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	comment.Id = primitive.NewObjectID().Hex()
	now := time.Now()
	comment.CreatedAt = now
	comment.UpdatedAt = now

	_, err := coll.InsertOne(ctx, comment)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return CommentDb{}, ErrDuplicatedRecord
		}
		return CommentDb{}, err
	}

	return comment, nil
}

func (db *DB) UpdateComment(ctx context.Context, commentDb CommentDb, userId string) (CommentDb, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentDb.Id, "userId": userId}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"comment":         commentDb.Comment,
			"seasonsComments": commentDb.SeasonsComments,
			"updatedAt":       now,
		},
	}

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	var updatedComment CommentDb
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedComment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return CommentDb{}, ErrRecordNotFound
		}
		return CommentDb{}, err
	}
	return updatedComment, nil
}

func (db *DB) DeleteComment(ctx context.Context, commentId, userId string) (int64, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentId, "userId": userId}
	result, err := coll.DeleteOne(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

func (db *DB) DeleteCommentsByTitleId(ctx context.Context, titleId string) (int64, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId}
	result, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
