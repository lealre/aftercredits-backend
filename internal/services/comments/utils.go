package comments

import (
	"errors"
	"net/http"
)

var (
	ErrCommentAlreadyExists       = errors.New("user comment already exists for this title")
	ErrCommentNotFound            = errors.New("comment not found")
	ErrCommentIsNull              = errors.New("comment cannot be empty")
	ErrSeasonValueInvalid         = errors.New("season value is invalid")
	ErrSeasonDoesNotExist         = errors.New("season does not exist for this title")
	ErrSeasonCommentAlreadyExists = errors.New("season comment already exists for this title")
)

var ErrorMap = map[error]int{
	ErrCommentAlreadyExists:       http.StatusConflict,
	ErrCommentNotFound:            http.StatusNotFound,
	ErrCommentIsNull:              http.StatusBadRequest,
	ErrSeasonValueInvalid:         http.StatusBadRequest,
	ErrSeasonDoesNotExist:         http.StatusBadRequest,
	ErrSeasonCommentAlreadyExists: http.StatusConflict,
}
