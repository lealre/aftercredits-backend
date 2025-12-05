package ratings

import (
	"errors"
	"net/http"
)

var (
	ErrRatingAlreadyExists = errors.New("user rating already exists for this title")
)

var ErrorMap = map[error]int{
	ErrRatingAlreadyExists: http.StatusConflict,
}
