package users

import (
	"errors"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/lealre/movies-backend/internal/validate"
)

var (
	ErrCredentialsAlreadyExists = errors.New("username or email already exists")
	ErrInvalidEmail             = errors.New("email format is not valid")
	ErrInvalidEmailSize         = errors.New("email is too long")
	ErrInvalidNameSize          = errors.New("name is too long")
	ErrInvalidUsernameSize      = errors.New("username must be between 3 and 32 characters")
	ErrInvalidUsername          = errors.New("username must contain only letters, numbers, '-' or '_'")
	ErrInvalidPassword          = errors.New("password must be between 12 and 72 characters")
	ErrInvalidCurrentPassword   = errors.New("current password is incorrect")
	ErrUserNotFound             = errors.New("user not found")
)

var ErrorMap = map[error]int{
	ErrInvalidUsername:          http.StatusBadRequest,
	ErrInvalidEmail:             http.StatusBadRequest,
	ErrInvalidEmailSize:         http.StatusBadRequest,
	ErrInvalidNameSize:          http.StatusBadRequest,
	ErrInvalidUsernameSize:      http.StatusBadRequest,
	ErrInvalidPassword:          http.StatusBadRequest,
	ErrInvalidCurrentPassword:   http.StatusUnauthorized,
	ErrCredentialsAlreadyExists: http.StatusConflict,
	ErrUserNotFound:             http.StatusNotFound,
}

// Password policy. The lower bound is a real floor for a public login page; the
// upper bound is bcrypt's hard limit — it silently ignores bytes past 72, so a
// longer value must be rejected rather than accepted and truncated.
const (
	MinPasswordLen = 12
	MaxPasswordLen = 72
)

// validatePassword enforces the length policy. Length is counted in bytes,
// deliberately: bcrypt's 72 limit is a byte limit, so a multi-byte password
// that is 72 runes could still overflow it.
func validatePassword(password string) error {
	if len(password) < MinPasswordLen || len(password) > MaxPasswordLen {
		return ErrInvalidPassword
	}
	return nil
}

// validateUserStrings enforces the length ceilings and character rules for the
// user-controlled fields, so an over-long or control-character-laden value is a
// clean 400 rather than a DB CHECK-constraint 500. Empty username/email are
// allowed (validated only when present), matching the column defaults.
func validateUserStrings(name, email, username string) error {
	if validate.TooLong(name, validate.NameMax) || validate.HasControlChars(name) {
		return ErrInvalidNameSize
	}
	if email != "" {
		if validate.TooLong(email, validate.EmailMax) {
			return ErrInvalidEmailSize
		}
		if !IsValidEmail(email) {
			return ErrInvalidEmail
		}
	}
	if username != "" {
		if utf8.RuneCountInString(username) < validate.UsernameMin || validate.TooLong(username, validate.UsernameMax) {
			return ErrInvalidUsernameSize
		}
		if !IsValidUsername(username) {
			return ErrInvalidUsername
		}
	}
	return nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}
