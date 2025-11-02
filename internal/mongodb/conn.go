package mongodb

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// --- DRY helpers: shared client and CRUD functions ---

var (
	mongoClient     *mongo.Client
	mongoClientOnce sync.Once
)

// connectMongo connects to MongoDB using MONGODB_URI and verifies the connection with a ping.
func ConnectMongo(ctx context.Context) *mongo.Client {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		fmt.Fprintln(os.Stderr, "MONGODB_URI is required (e.g. mongodb://localhost:27017)")
		os.Exit(2)
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mongo connect error: %v\n", err)
		os.Exit(1)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		fmt.Fprintf(os.Stderr, "mongo ping error: %v\n", err)
		os.Exit(1)
	}

	return client
}

func getMongoClient(ctx context.Context) *mongo.Client {
	mongoClientOnce.Do(func() {
		mongoClient = ConnectMongo(ctx)
	})
	return mongoClient
}

// func getDatabaseName() string {
// 	name := os.Getenv("MONGODB_DB")
// 	if name == "" {
// 		name = "brunan"
// 	}
// 	return name
// }

func GetTitlesCollection(ctx context.Context) *mongo.Collection {
	client := getMongoClient(ctx)
	return client.Database(getDatabaseName()).Collection("titles")
}

func GetUsersCollection(ctx context.Context) *mongo.Collection {
	client := getMongoClient(ctx)
	return client.Database(getDatabaseName()).Collection("users")
}

func GetRatingsCollection(ctx context.Context) *mongo.Collection {
	client := getMongoClient(ctx)
	return client.Database(getDatabaseName()).Collection("ratings")
}

func GetCommentsCollection(ctx context.Context) *mongo.Collection {
	client := getMongoClient(ctx)
	return client.Database(getDatabaseName()).Collection("comments")
}
