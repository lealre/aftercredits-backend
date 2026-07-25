package titles

import (
	"errors"
	"net/http"
)

// Service-level errors for the titles package. Handlers translate these to HTTP
// status codes via ErrorMap (same pattern as groups/ratings/comments/users) —
// error strings and their statuses live here, not in the handlers.
var (
	ErrTitleNotFound      = errors.New("title not found")
	ErrTitleAlreadyExists = errors.New("title already added")
	ErrInvalidIMDbURL     = errors.New("invalid IMDb title URL")
)

var ErrorMap = map[error]int{
	ErrTitleNotFound:      http.StatusNotFound,
	ErrTitleAlreadyExists: http.StatusBadRequest,
	ErrInvalidIMDbURL:     http.StatusBadRequest,
}
