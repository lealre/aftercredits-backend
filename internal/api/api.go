package api

import (
	"github.com/lealre/movies-backend/internal/store"
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
	Db       store.Store
	Secret   *string
	Provider titleprovider.Provider
}

func NewAPI(db store.Store, provider titleprovider.Provider) *API {
	return &API{Db: db, Provider: provider}
}

var PublicPaths = map[string]bool{
	"POST /login": true,
	"POST /users": true,
}
