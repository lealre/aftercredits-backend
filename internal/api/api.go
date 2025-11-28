package api

import (
	"github.com/lealre/movies-backend/internal/mongodb"
)

type ErrorResponse struct {
	StatusCode   int    `json:"status_code"`
	ErrorMessage string `json:"error_message"`
}

type DefaultResponse struct {
	Message string `json:"message"`
}

type API struct {
	Db     *mongodb.DB
	Secret *string
}

func NewAPI(db *mongodb.DB) *API {
	return &API{Db: db}
}

var PublicPaths = map[string]bool{
	"POST /login": true,
	"POST /users": true,
}
