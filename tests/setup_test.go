package tests

import (
	"context"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/server"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	testClient *mongo.Client
	testServer *httptest.Server
)

const TEST_DB_NAME = "testDb"

func TestMain(m *testing.M) {
	ctx := context.Background()

	os.Setenv("MONGODB_DB", TEST_DB_NAME)
	req := testcontainers.ContainerRequest{
		Image:        "mongo:7.0",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp"),
	}
	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start mongo container: %v", err)
	}

	endpoint, err := mongoC.Endpoint(ctx, "")
	if err != nil {
		log.Fatalf("failed to get mongo endpoint: %v", err)
	}
	uri := "mongodb://" + endpoint

	testClient, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("failed to connect to test mongo: %v", err)
	}

	handler := server.NewServer(testClient)
	testServer = httptest.NewServer(handler)

	code := m.Run()

	// Cleanup
	testServer.Close()
	_ = testClient.Disconnect(ctx)
	_ = mongoC.Terminate(ctx)

	os.Exit(code)
}

func resetDB(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)

	collections, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		t.Fatalf("failed to list collections: %v", err)
	}

	for _, coll := range collections {
		if err := db.Collection(coll).Drop(ctx); err != nil {
			t.Fatalf("failed to drop collection %s: %v", coll, err)
		}
	}

	// Recreate indexes after dropping collections
	createIndexes(t, db)
}

func createIndexes(t *testing.T, db *mongo.Database) {
	t.Helper()

	ctx := context.Background()
	usersColl := db.Collection(mongodb.UsersCollection)
	ratingsColl := db.Collection(mongodb.RatingsCollection)
	commentsColl := db.Collection(mongodb.CommentsCollection)

	// Create unique index on email (case-insensitive)
	// Exclude empty strings and null values from uniqueness constraint
	usersEmailIndexName := "email_unique"
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
	_, err := usersColl.Indexes().CreateOne(ctx, emailIndex)
	if err != nil {
		t.Fatalf("failed to create unique index on email: %v", err)
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
	_, err = usersColl.Indexes().CreateOne(ctx, usernameIndex)
	if err != nil {
		t.Fatalf("failed to create unique index on username: %v", err)
	}

	// Create unique index on ratings collection
	ratingsIndexName := "userId_and_titleId_unique"
	ratingsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(ratingsIndexName),
	}
	_, err = ratingsColl.Indexes().CreateOne(ctx, ratingsIndex)
	if err != nil {
		t.Fatalf("failed to create unique index on ratings collection: %v", err)
	}

	// Create unique index on comments collection
	commentsIndexName := "userId_and_titleId_unique"
	commentsIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "titleId", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName(commentsIndexName),
	}
	_, err = commentsColl.Indexes().CreateOne(ctx, commentsIndex)
	if err != nil {
		t.Fatalf("failed to create unique index on comments collection: %v", err)
	}
}
