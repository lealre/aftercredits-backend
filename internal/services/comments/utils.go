package comments

import (
	"errors"
	"net/http"

	"github.com/lealre/movies-backend/internal/validate"
)

var (
	ErrCommentAlreadyExists       = errors.New("user comment already exists for this title")
	ErrCommentNotFound            = errors.New("comment not found")
	ErrCommentIsNull              = errors.New("comment cannot be empty")
	ErrCommentTooLong             = errors.New("comment is too long")
	ErrInvalidSeasonValue         = errors.New("season value is invalid")
	ErrSeasonRequired             = errors.New("season number is required for TV series comments")
	ErrSeasonDoesNotExist         = errors.New("season does not exist for this title")
	ErrSeasonCommentAlreadyExists = errors.New("season comment already exists for this title")
)

var ErrorMap = map[error]int{
	ErrCommentAlreadyExists:       http.StatusConflict,
	ErrCommentNotFound:            http.StatusNotFound,
	ErrCommentIsNull:              http.StatusBadRequest,
	ErrCommentTooLong:             http.StatusBadRequest,
	ErrInvalidSeasonValue:         http.StatusBadRequest,
	ErrSeasonRequired:             http.StatusBadRequest,
	ErrSeasonDoesNotExist:         http.StatusBadRequest,
	ErrSeasonCommentAlreadyExists: http.StatusConflict,
}

// validateCommentText enforces the length ceiling and rejects control
// characters (newlines allowed — a comment is prose). Empty is handled by the
// callers' existing ErrCommentIsNull check.
func validateCommentText(text string) error {
	if validate.TooLong(text, validate.CommentMax) {
		return ErrCommentTooLong
	}
	if validate.HasControlChars(text) {
		return ErrCommentIsNull
	}
	return nil
}
