package tests

import (
	"context"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func getGroup(t *testing.T, groupId string) mongodb.GroupDb {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.GroupsCollection)
	var group mongodb.GroupDb
	err := coll.FindOne(ctx, bson.M{"_id": groupId}).Decode(&group)
	require.NoError(t, err, "error queryind a group from db")

	return group

}
