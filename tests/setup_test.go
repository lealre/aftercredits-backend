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
	if err := mongodb.CreateAllIndexes(ctx, db, false); err != nil {
		t.Fatalf("failed to create indexes: %v", err)
	}
}
