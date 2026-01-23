package comments

import (
	"errors"
	"net/http"
)

var (
	ErrCommentAlreadyExists       = errors.New("user comment already exists for this title")
	ErrCommentNotFound            = errors.New("comment not found")
	ErrCommentIsNull              = errors.New("comment cannot be empty")
	ErrInvalidSeasonValue         = errors.New("season value is invalid")
	ErrSeasonRequired             = errors.New("season number is required for TV series comments")
	ErrSeasonDoesNotExist         = errors.New("season does not exist for this title")
	ErrSeasonCommentAlreadyExists = errors.New("season comment already exists for this title")
)

var ErrorMap = map[error]int{
	ErrCommentAlreadyExists:       http.StatusConflict,
	ErrCommentNotFound:            http.StatusNotFound,
	ErrCommentIsNull:              http.StatusBadRequest,
	ErrInvalidSeasonValue:         http.StatusBadRequest,
	ErrSeasonRequired:             http.StatusBadRequest,
	ErrSeasonDoesNotExist:         http.StatusBadRequest,
	ErrSeasonCommentAlreadyExists: http.StatusConflict,
}
