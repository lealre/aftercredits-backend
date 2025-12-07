package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func addComment(t *testing.T, newComment comments.NewComment, innerToken string) *http.Response {
	jsonData, err := json.Marshal(newComment)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/comments",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+innerToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func getComment(t *testing.T, commentId string) mongodb.CommentDb {
	ctx := context.Background()
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.CommentsCollection)

	var comment mongodb.CommentDb
	err := coll.FindOne(ctx, bson.M{"_id": commentId}).Decode(&comment)
	require.NoError(t, err, "error querying a comment from db")

	return comment
}
