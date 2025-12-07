package comments

import (
	"errors"
	"net/http"
)

var (
	ErrCommentAlreadyExists = errors.New("user comment already exists for this title")
	ErrCommentNotFound      = errors.New("comment not found")
	ErrCommentIsNull        = errors.New("comment cannot be empty")
)

var ErrorMap = map[error]int{
	ErrCommentAlreadyExists: http.StatusConflict,
	ErrCommentNotFound:      http.StatusNotFound,
	ErrCommentIsNull:        http.StatusBadRequest,
}
