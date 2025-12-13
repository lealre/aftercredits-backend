package auth

import (
	"errors"
	"net/http"
)

var (
	ErrTokenSigningMethod    = errors.New("unexpected signing method")
	ErrInvalidToken          = errors.New("invalid token")
	ErrTokenExpired          = errors.New("token has expired")
	ErrTokenWithNoSubject    = errors.New("token has no subject")
	ErrNoAuthorizationHeader = errors.New("no 'Authorization' header found")
	ErrMalformedAuthHeader   = errors.New("token must start with 'Bearer '")
	ErrNoTokenInAuthHeader   = errors.New("token must start with 'Bearer '")
	ErrInvalidCredentials    = errors.New("invalid credentials")
)

var ErrorsMap = map[error]int{
	ErrTokenSigningMethod:    http.StatusUnauthorized,
	ErrInvalidToken:          http.StatusUnauthorized,
	ErrTokenExpired:          http.StatusUnauthorized,
	ErrTokenWithNoSubject:    http.StatusUnauthorized,
	ErrNoAuthorizationHeader: http.StatusUnauthorized,
	ErrMalformedAuthHeader:   http.StatusUnauthorized,
	ErrNoTokenInAuthHeader:   http.StatusUnauthorized,
	ErrInvalidCredentials:    http.StatusUnauthorized,
}
