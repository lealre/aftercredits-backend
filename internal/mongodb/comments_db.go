package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
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

func (db *DB) GetCommentsByTitleId(ctx context.Context, titleId string, usersFromGroup []string) ([]models.Comment, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId, "userId": bson.M{"$in": usersFromGroup}}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return []models.Comment{}, err
	}
	defer cursor.Close(ctx)

	var commentsDb []CommentDb
	if err = cursor.All(ctx, &commentsDb); err != nil {
		return []models.Comment{}, err
	}

	comments := make([]models.Comment, len(commentsDb))
	for i, c := range commentsDb {
		comments[i] = commentDbToModel(c)
	}

	return comments, nil
}

func (db *DB) GetUserCommentByTitleId(ctx context.Context, titleId string, userId string) (models.Comment, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"titleId": titleId, "userId": userId}

	var commentDb CommentDb
	err := coll.FindOne(ctx, filter).Decode(&commentDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Comment{}, store.ErrRecordNotFound
		}
		return models.Comment{}, err
	}

	return commentDbToModel(commentDb), nil
}

func (db *DB) GetCommentById(ctx context.Context, commentId string, userId string) (models.Comment, error) {
	coll := db.Collection(CommentsCollection)

	filter := bson.M{"_id": commentId, "userId": userId}

	var commentDb CommentDb
	err := coll.FindOne(ctx, filter).Decode(&commentDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Comment{}, store.ErrRecordNotFound
		}
		return models.Comment{}, err
	}

	return commentDbToModel(commentDb), nil
}

func (db *DB) AddComment(ctx context.Context, comment models.Comment) (models.Comment, error) {
	coll := db.Collection(CommentsCollection)

	commentDb := commentModelToDb(comment)
	commentDb.Id = primitive.NewObjectID().Hex()
	now := time.Now()
	commentDb.CreatedAt = now
	commentDb.UpdatedAt = now

	_, err := coll.InsertOne(ctx, commentDb)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Comment{}, store.ErrDuplicatedRecord
		}
		return models.Comment{}, err
	}

	return commentDbToModel(commentDb), nil
}

func (db *DB) UpdateComment(ctx context.Context, comment models.Comment, userId string) (models.Comment, error) {
	coll := db.Collection(CommentsCollection)

	commentDb := commentModelToDb(comment)

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

	var updatedCommentDb CommentDb
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedCommentDb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Comment{}, store.ErrRecordNotFound
		}
		return models.Comment{}, err
	}
	return commentDbToModel(updatedCommentDb), nil
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
