package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/ratings"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func getRating(t *testing.T, ratingId string) mongodb.RatingDb {
	db := testClient.Database(TEST_DB_NAME)
	coll := db.Collection(mongodb.RatingsCollection)

	var rating mongodb.RatingDb
	err := coll.FindOne(context.Background(), bson.M{"_id": ratingId}).Decode(&rating)
	require.NoError(t, err, "error queryind a rating from db")

	return rating
}

func addRating(t *testing.T, newRating ratings.NewRating, innerToken string) *http.Response {
	jsonData, err := json.Marshal(newRating)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		testServer.URL+"/ratings",
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
