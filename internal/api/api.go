package api

import (
	"go.mongodb.org/mongo-driver/mongo"
)

type API struct {
	DB *mongo.Client
}

func NewAPI(db *mongo.Client) *API {
	return &API{DB: db}
}
