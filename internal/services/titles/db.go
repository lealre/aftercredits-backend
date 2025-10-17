package titles

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getTitleByID fetches a title document by its _id
func GetTitleByID(ctx context.Context, id string) (bson.M, error) {
	coll := mongodb.GetTitlesCollection(ctx)
	var out bson.M
	if err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&out); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongodb.ErrRecordNotFound
		}
		return nil, err
	}
	return out, nil
}

// addTitle inserts a document; returns duplicate key error if it already exists
func AddTitle(ctx context.Context, doc map[string]any) error {
	if doc == nil {
		return fmt.Errorf("doc is nil")
	}
	if _, ok := doc["_id"]; !ok {
		return fmt.Errorf("doc missing _id")
	}
	coll := mongodb.GetTitlesCollection(ctx)
	_, err := coll.InsertOne(ctx, doc)
	return err
}

// DeleteTitleByID deletes a document by _id and returns whether a doc was removed
func DeleteTitleByID(ctx context.Context, id string) (bool, error) {
	coll := mongodb.GetTitlesCollection(ctx)
	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// GetAllTitles fetches all title documents from the collection
func GetAllTitles(ctx context.Context) (*mongo.Cursor, error) {
	coll := mongodb.GetTitlesCollection(ctx)
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return cursor, nil
}

func SetWatched(ctx context.Context, id string, watched bool) (*Title, error) {
	coll := mongodb.GetTitlesCollection(ctx)

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	var updatedTitle imdb.Title
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"watched": watched}},
		opts,
	).Decode(&updatedTitle)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongodb.ErrRecordNotFound
		}
		return nil, err
	}
	updatedTitleDb := MapDbTitleToApiTitle(updatedTitle)

	return &updatedTitleDb, nil
}

// CascadeDeleteTitle deletes a title and all its related data (ratings)
func CascadeDeleteTitle(ctx context.Context, titleId string) (int64, error) {
	// Delete all related ratings first
	deletedRatingsCount, err := ratings.DeleteRatingsByTitleId(ctx, titleId)
	if err != nil {
		log.Printf("Error deleting related ratings: %v", err)
		return 0, fmt.Errorf("failed to delete related ratings: %w", err)
	}

	// Delete the title
	_, err = DeleteTitleByID(ctx, titleId)
	if err != nil {
		log.Printf("Error deleting title: %v", err)
		return 0, fmt.Errorf("failed to delete title: %w", err)
	}

	return deletedRatingsCount, nil
}
