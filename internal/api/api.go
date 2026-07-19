package api

import (
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

type ErrorResponse struct {
	StatusCode   int    `json:"statusCode"`
	ErrorMessage string `json:"errorMessage"`
}

type DefaultResponse struct {
	Message string `json:"message"`
}

type API struct {
	Db       *mongodb.DB
	Secret   *string
	Provider titleprovider.Provider
}

func NewAPI(db *mongodb.DB, provider titleprovider.Provider) *API {
	return &API{Db: db, Provider: provider}
}

var PublicPaths = map[string]bool{
	"POST /login": true,
	"POST /users": true,
}
