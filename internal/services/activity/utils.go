package activity

import (
	"errors"
	"net/http"
)

var ErrInvalidSeq = errors.New("seq must be a positive number")

var ErrorMap = map[error]int{
	ErrInvalidSeq: http.StatusBadRequest,
}
