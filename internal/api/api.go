package api

import (
	"github.com/lealre/movies-backend/internal/mongodb"
)

type API struct {
	Db *mongodb.DB
}

func NewAPI(db *mongodb.DB) *API {
	return &API{Db: db}
}
