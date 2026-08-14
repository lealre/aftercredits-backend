package api

import (
	"github.com/lealre/movies-backend/internal/services/activity"
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

	// Stream is nil unless ACTIVITY_FEED_ENABLED is on: the server only builds
	// it — and only registers the two routes that use it — inside the flag
	// branch, so with the feature off there is no hub, no ticket store and no
	// handler that could reach them.
	Stream *activity.Streamer
}

func NewAPI(db store.Store, provider titleprovider.Provider) *API {
	return &API{Db: db, Provider: provider}
}

var PublicPaths = map[string]bool{
	"POST /login": true,
	"POST /users": true,
	// Public to AuthMiddleware only: EventSource cannot send an Authorization
	// header, so the stream authenticates with a single-use ticket inside the
	// handler instead. POST /activity/stream-ticket, which mints those tickets,
	// is deliberately absent from this map — it needs a real authenticated
	// user, or anyone could mint a ticket for anyone.
	"GET /activity/stream": true,
}
