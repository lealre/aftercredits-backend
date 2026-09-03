package groups

import (
	"errors"
	"net/http"

	"github.com/lealre/movies-backend/internal/validate"
)

var (
	ErrGroupNotOwnedByUser                 = errors.New("only the group owner can perform this action")
	ErrGroupNotFound                       = errors.New("group not found")
	ErrGroupNameInvalid                    = errors.New("group name is invalid")
	ErrGroupNameTooLong                    = errors.New("group name is too long")
	ErrGroupDescriptionTooLong             = errors.New("group description is too long")
	ErrGroupDuplicatedName                 = errors.New("a group with this name already exists")
	ErrGroupQuotaExceeded                  = errors.New("group creation limit reached")
	ErrTitleAlreadyInGroup                 = errors.New("title is already in group")
	ErrTitleNotInGroup                     = errors.New("title not found in group")
	ErrGroupTitleQuotaExceeded             = errors.New("this group has reached its title limit")
	ErrUpdatingWatchedAtWhenWatchedIsFalse = errors.New("cannot update watchedAt when watched is set to false")
	ErrInvalidSeasonValue                  = errors.New("season value is invalid")
	ErrSeasonDoesNotExist                  = errors.New("season does not exist for this title")
	ErrOwnerCannotLeaveGroup               = errors.New("the group owner cannot leave; delete the group instead")
)

var ErrorMap = map[error]int{
	ErrGroupNotOwnedByUser:                 http.StatusForbidden,
	ErrGroupNotFound:                       http.StatusNotFound,
	ErrGroupNameInvalid:                    http.StatusBadRequest,
	ErrGroupNameTooLong:                    http.StatusBadRequest,
	ErrGroupDescriptionTooLong:             http.StatusBadRequest,
	ErrGroupDuplicatedName:                 http.StatusBadRequest,
	ErrGroupQuotaExceeded:                  http.StatusTooManyRequests,
	ErrTitleAlreadyInGroup:                 http.StatusConflict,
	ErrTitleNotInGroup:                     http.StatusNotFound,
	ErrGroupTitleQuotaExceeded:             http.StatusTooManyRequests,
	ErrUpdatingWatchedAtWhenWatchedIsFalse: http.StatusBadRequest,
	ErrInvalidSeasonValue:                  http.StatusBadRequest,
	ErrSeasonDoesNotExist:                  http.StatusBadRequest,
	ErrOwnerCannotLeaveGroup:               http.StatusForbidden,
}

// validateGroupStrings enforces the length ceilings and control-character rules
// on the group name and description. An empty name is invalid (unchanged).
func validateGroupStrings(name, description string) error {
	if name == "" {
		return ErrGroupNameInvalid
	}
	if validate.TooLong(name, validate.GroupNameMax) {
		return ErrGroupNameTooLong
	}
	if validate.HasControlChars(name) {
		return ErrGroupNameInvalid
	}
	if validate.TooLong(description, validate.GroupDescMax) {
		return ErrGroupDescriptionTooLong
	}
	return nil
}
