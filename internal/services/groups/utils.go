package groups

import (
	"errors"
	"net/http"
)

var (
	ErrGroupNotOwnedByUser = errors.New("just the owner of the group can perform this action")
	ErrGroupNotFound       = errors.New("group not found")
)

var ErrorMap = map[error]int{
	ErrGroupNotOwnedByUser: http.StatusForbidden,
	ErrGroupNotFound:       http.StatusNotFound,
}
