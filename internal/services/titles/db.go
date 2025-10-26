package titles

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lealre/movies-backend/internal/imdb"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

func DeleteTitleByID(ctx context.Context, id string) (bool, error) {
	coll := mongodb.GetTitlesCollection(ctx)
	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func GetTitlesDb(ctx context.Context, args ...any) ([]Title, error) {
	coll := mongodb.GetTitlesCollection(ctx)

	filter, opts := mongodb.ResolveFilterAndOptionsSearch(args...)
	cursor, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Iterate through the cursor and map each title to a API title struct
	var allMovies []Title
	for cursor.Next(ctx) {
		var title imdb.Title
		if err := cursor.Decode(&title); err != nil {
			return []Title{}, err
		}

		movie := MapDbTitleToApiTitle(title)
		allMovies = append(allMovies, movie)
	}

	if err := cursor.Err(); err != nil {
		return []Title{}, err
	}

	return allMovies, nil
}

func CountTotalTitlesDb(ctx context.Context, args ...any) (int, error) {
	coll := mongodb.GetTitlesCollection(ctx)

	filter, _ := mongodb.ResolveFilterAndOptionsSearch(args...)
	totalTitles, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(totalTitles), nil
}

func SetWatched(ctx context.Context, id string, watched *bool, watchedAt *FlexibleDate) (*Title, error) {
	coll := mongodb.GetTitlesCollection(ctx)

	// Use FindOneAndUpdate to get the updated document
	opts := options.FindOneAndUpdate()
	opts.SetReturnDocument(options.After) // Return the document after update

	updateDoc := bson.M{}

	// Update watched field if provided
	if watched != nil {
		updateDoc["watched"] = *watched
	}

	// Update watchedAt field if provided
	if watchedAt != nil {
		if watchedAt.Time != nil {
			updateDoc["watchedAt"] = *watchedAt.Time
		} else {
			// If watchedAt is provided but Time is nil, set it to null in database
			updateDoc["watchedAt"] = nil
		}
	}

	// Always update the updatedAt timestamp if any field is being updated
	if len(updateDoc) > 0 {
		now := time.Now()
		updateDoc["updatedAt"] = now
	}

	if len(updateDoc) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	var updatedTitle imdb.Title
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": updateDoc},
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
		return 0, err
	}

	// Delete the title
	_, err = DeleteTitleByID(ctx, titleId)
	if err != nil {
		return 0, err
	}

	return deletedRatingsCount, nil
}
