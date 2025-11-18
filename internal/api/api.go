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
	Db *mongodb.DB
}

func NewAPI(db *mongodb.DB) *API {
	return &API{Db: db}
}
