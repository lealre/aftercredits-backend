package mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var ErrRecordNotFound = errors.New("record not found in the database")

const (
	TitlesCollection   = "titles"
	TitlesCollectionG  = "titlesG"
	UsersCollection    = "users"
	RatingsCollection  = "ratings"
	CommentsCollection = "comments"
	GroupsCollection   = "groups"
)

type DB struct {
	client *mongo.Client
	dbName string
}

func NewDB(client *mongo.Client) *DB {
	return &DB{client: client, dbName: getDatabaseName()}
}

func (db *DB) Collection(name string) *mongo.Collection {
	return db.client.Database(db.dbName).Collection(name)
}

func (db *DB) GetDatabaseName() string {
	return db.dbName
}

func Connect(ctx context.Context) (*mongo.Client, error) {
	uri := getMongoURI()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect error: %v", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping error: %v", err)
	}

	return client, nil
}

func getMongoURI() string {
	user := os.Getenv("MONGO_USER")
	password := os.Getenv("MONGO_PASSWORD")
	host := os.Getenv("MONGO_HOST")
	port := os.Getenv("MONGO_PORT")

	if host == "" {
		host = "localhost"
	}

	if port == "" {
		port = "27017"
	}

	// If credentials are provided, use them with authSource=admin
	// Otherwise, connect without authentication
	if user != "" && password != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s:%s/?authSource=admin", user, password, host, port)
	}

	return fmt.Sprintf("mongodb://%s:%s", host, port)
}

func getDatabaseName() string {
	name := os.Getenv("MONGODB_DB")
	if name == "" {
		name = "brunan"
	}
	return name
}

func ResolveFilterAndOptionsSearch(args ...any) (bson.M, []*options.FindOptions) {
	filter := bson.M{}
	var opts []*options.FindOptions

	for _, arg := range args {
		switch v := arg.(type) {
		case bson.M:
			filter = v
		case *options.FindOptions:
			opts = append(opts, v)
		default:
			// Just ignore if no args match
		}
	}

	return filter, opts
}
