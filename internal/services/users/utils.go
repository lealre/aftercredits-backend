package users

import (
	"errors"
	"net/http"
	"regexp"
)

var (
	ErrCredentialsAlreadyExists = errors.New("username or email already exists")
	ErrInvalidEmail             = errors.New("email format is not valid")
	ErrInvalidUsernameSize      = errors.New("username must have at least 3 charscters")
	ErrInvalidUsername          = errors.New("username must contains just letters, numbers, '-' or '_'")
	ErrInvalidPassword          = errors.New("invalid passsword")
)

var ErrorMap = map[error]int{
	ErrInvalidUsername:          http.StatusBadRequest,
	ErrInvalidEmail:             http.StatusBadRequest,
	ErrInvalidUsernameSize:      http.StatusBadRequest,
	ErrInvalidPassword:          http.StatusBadRequest,
	ErrCredentialsAlreadyExists: http.StatusConflict,
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}
