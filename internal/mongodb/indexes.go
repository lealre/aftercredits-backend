package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DeleteAllIndexes deletes all indexes from all collections in the database
// (except the default _id_ index which cannot be deleted)
func DeleteAllIndexes(ctx context.Context, db *mongo.Database) error {
	// Get all collections in the database
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, collName := range collections {
		coll := db.Collection(collName)

		// List all indexes for this collection
		cursor, err := coll.Indexes().List(ctx)
		if err != nil {
			return fmt.Errorf("failed to list indexes for collection '%s': %w", collName, err)
		}

		// Iterate through indexes and delete them (except _id_ which is the default and cannot be deleted)
		for cursor.Next(ctx) {
			var index bson.M
			if err := cursor.Decode(&index); err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("failed to decode index for collection '%s': %w", collName, err)
			}

			indexName, ok := index["name"].(string)
			if !ok {
				continue
			}

			// Skip the default _id_ index as it cannot be deleted
			if indexName == "_id_" {
				continue
			}

			// Delete the index
			_, err := coll.Indexes().DropOne(ctx, indexName)
			if err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("failed to delete index '%s' from collection '%s': %w", indexName, collName, err)
			}
			fmt.Printf("🗑️  Deleted index '%s' from collection '%s'\n", indexName, collName)
		}

		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return fmt.Errorf("cursor error for collection '%s': %w", collName, err)
		}
		cursor.Close(ctx)
	}

	return nil
}

// CreateAllIndexes creates all indexes for users, ratings, and comments collections
func CreateAllIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	// Create indexes for users collection
	if err := CreateUserIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create user indexes: %w", err)
	}

	// Create indexes for comments collection
	if err := CreateCommentIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create comment indexes: %w", err)
	}

	// Create indexes for groups collection
	if err := CreateGroupIndexes(ctx, db, reset); err != nil {
		return fmt.Errorf("failed to create group indexes: %w", err)
	}

	return nil
}

// CreateUserIndexes creates indexes for the users collection
func CreateUserIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(UsersCollection)
	usersEmailIndexName := "email_unique"

	// Create unique index on email (case-insensitive)
	// Exclude empty strings and null values from uniqueness constraint
	emailIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(usersEmailIndexName).
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{
				"$and": []bson.M{
					{"email": bson.M{"$type": "string"}},
					{"email": bson.M{"$gt": ""}},
				},
			}),
	}
	if err := createIndexIfNotExists(ctx, coll, emailIndex, usersEmailIndexName, reset); err != nil {
		return err
	}

	// Create unique index on username (case-insensitive)
	// Exclude empty strings and null values from uniqueness constraint
	usersUsernameIndexName := "username_unique"
	usernameIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(usersUsernameIndexName).
			SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}).
			SetPartialFilterExpression(bson.M{
				"$and": []bson.M{
					{"username": bson.M{"$type": "string"}},
					{"username": bson.M{"$gt": ""}},
				},
			}),
	}
	if err := createIndexIfNotExists(ctx, coll, usernameIndex, usersUsernameIndexName, reset); err != nil {
		return err
	}

	return nil
}

// CreateCommentIndexes creates indexes for the comments collection
func CreateCommentIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(CommentsCollection)
	commentsIndexName := "userId_and_titleId_unique"

	// Create unique index on userId and titleId
	commentsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(commentsIndexName),
	}
	if err := createIndexIfNotExists(ctx, coll, commentsIndex, commentsIndexName, reset); err != nil {
		return err
	}

	return nil
}

// CreateGroupIndexes creates indexes for the groups collection
func CreateGroupIndexes(ctx context.Context, db *mongo.Database, reset bool) error {
	coll := db.Collection(GroupsCollection)
	groupsIndexName := "ownerId_and_name_unique"

	// Create unique index on ownerId and name
	// Exclude null values and empty strings from uniqueness constraint
	groupsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(groupsIndexName).
			SetPartialFilterExpression(bson.M{
				"$and": []bson.M{
					{"ownerId": bson.M{"$type": "string"}},
					{"ownerId": bson.M{"$gt": ""}},
					{"name": bson.M{"$type": "string"}},
					{"name": bson.M{"$gt": ""}},
				},
			}),
	}
	if err := createIndexIfNotExists(ctx, coll, groupsIndex, groupsIndexName, reset); err != nil {
		return err
	}

	return nil
}

// createIndexIfNotExists checks if an index exists and creates it if it doesn't
// If reset is true, it will delete the existing index and recreate it
func createIndexIfNotExists(ctx context.Context, coll *mongo.Collection, indexModel mongo.IndexModel, indexName string, reset bool) error {
	// List existing indexes
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}
	defer cursor.Close(ctx)

	// Check if index already exists
	indexExists := false
	for cursor.Next(ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			return fmt.Errorf("failed to decode index: %w", err)
		}

		if name, ok := index["name"].(string); ok && name == indexName {
			indexExists = true
			break
		}
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	if indexExists {
		if !reset {
			fmt.Printf("ℹ️  Index '%s' already exists on collection '%s', skipping...\n", indexName, coll.Name())
			return nil
		}
		// Delete the existing index
		_, err := coll.Indexes().DropOne(ctx, indexName)
		if err != nil {
			return fmt.Errorf("failed to delete index '%s': %w", indexName, err)
		}
		fmt.Printf("🗑️  Deleted index '%s' on collection '%s'\n", indexName, coll.Name())
	}

	// Create the index
	_, err = coll.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create index '%s': %w", indexName, err)
	}

	fmt.Printf("✅ Created index '%s' on collection '%s'\n", indexName, coll.Name())
	return nil
}
