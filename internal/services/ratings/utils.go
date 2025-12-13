package ratings

import (
	"errors"
	"net/http"
)

var (
	ErrRatingAlreadyExists = errors.New("user rating already exists for this title")
	ErrRatingNotFound      = errors.New("rating not found")
	ErrInvalidNoteValue    = errors.New("rating note must be between 0 and 10")
)

var ErrorMap = map[error]int{
	ErrRatingAlreadyExists: http.StatusConflict,
	ErrRatingNotFound:      http.StatusNotFound,
	ErrInvalidNoteValue:    http.StatusBadRequest,
}
