package groups

import (
	"errors"
	"net/http"
)

var (
	ErrGroupNotOwnedByUser                 = errors.New("only the group owner can perform this action")
	ErrGroupNotFound                       = errors.New("group not found")
	ErrGroupNameInvalid                    = errors.New("group name is invalid")
	ErrGroupDuplicatedName                 = errors.New("a group with this name already exists")
	ErrTitleAlreadyInGroup                 = errors.New("title is already in group")
	ErrTitleNotInGroup                     = errors.New("title not found in group")
	ErrUpdatingWatchedAtWhenWatchedIsFalse = errors.New("cannot update watchedAt when watched is set to false")
	ErrInvalidSeasonValue                  = errors.New("season value is invalid")
	ErrSeasonDoesNotExist                  = errors.New("season does not exist for this title")
)

var ErrorMap = map[error]int{
	ErrGroupNotOwnedByUser:                 http.StatusForbidden,
	ErrGroupNotFound:                       http.StatusNotFound,
	ErrGroupNameInvalid:                    http.StatusBadRequest,
	ErrGroupDuplicatedName:                 http.StatusBadRequest,
	ErrTitleAlreadyInGroup:                 http.StatusConflict,
	ErrTitleNotInGroup:                     http.StatusNotFound,
	ErrUpdatingWatchedAtWhenWatchedIsFalse: http.StatusBadRequest,
	ErrInvalidSeasonValue:                  http.StatusBadRequest,
	ErrSeasonDoesNotExist:                  http.StatusBadRequest,
}
