package groups

import (
	"errors"
	"net/http"
)

var (
	ErrGroupNotOwnedByUser = errors.New("only the group owner can perform this action")
	ErrGroupNotFound       = errors.New("group not found")
	ErrGroupNameInvalid    = errors.New("group name is invalid")
	ErrGroupDuplicatedName = errors.New("a group with this name already exists")
)

var ErrorMap = map[error]int{
	ErrGroupNotOwnedByUser: http.StatusForbidden,
	ErrGroupNotFound:       http.StatusNotFound,
	ErrGroupNameInvalid:    http.StatusBadRequest,
	ErrGroupDuplicatedName: http.StatusBadRequest,
}
