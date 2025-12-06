package ratings

import (
	"errors"
	"net/http"
)

var (
	ErrRatingAlreadyExists = errors.New("user rating already exists for this title")
	ErrRatingNotFound      = errors.New("rating not found")
)

var ErrorMap = map[error]int{
	ErrRatingAlreadyExists: http.StatusConflict,
	ErrRatingNotFound:      http.StatusNotFound,
}
