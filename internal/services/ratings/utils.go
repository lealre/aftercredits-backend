package ratings

import (
	"errors"
	"net/http"
)

var (
	ErrRatingAlreadyExists       = errors.New("user rating already exists for this title")
	ErrRatingNotFound            = errors.New("rating not found")
	ErrInvalidNoteValue          = errors.New("rating note must be between 0 and 10")
	ErrSeasonRequired            = errors.New("season number is required for TV series ratings")
	ErrInvalidSeasonValue        = errors.New("season number must be greater than 0")
	ErrSeasonDoesNotExist        = errors.New("season does not exist for this title")
	ErrSeasonRatingAlreadyExists = errors.New("rating already exists for this season")
)

var ErrorMap = map[error]int{
	ErrRatingAlreadyExists:       http.StatusConflict,
	ErrRatingNotFound:            http.StatusNotFound,
	ErrInvalidNoteValue:          http.StatusBadRequest,
	ErrSeasonRequired:            http.StatusBadRequest,
	ErrInvalidSeasonValue:        http.StatusBadRequest,
	ErrSeasonDoesNotExist:        http.StatusBadRequest,
	ErrSeasonRatingAlreadyExists: http.StatusConflict,
}
