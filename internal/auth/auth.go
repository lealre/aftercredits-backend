package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lealre/movies-backend/internal/mongodb"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserKey contextKey = "user"

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID string, tokenSecret string, expiresIn time.Duration) (string, error) {
	claim := jwt.RegisteredClaims{
		Issuer:    "mytitles",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)

	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return string(signedToken), nil
}

func ValidateJWT(tokenString, tokenSecret string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrTokenSigningMethod
			}
			return []byte(tokenSecret), nil
		},
	)
	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", ErrInvalidToken
	}

	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return "", ErrTokenExpired
	}

	if claims.Subject == "" {
		return "", ErrTokenWithNoSubject
	}

	return claims.Subject, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	bearerToken := headers.Get("Authorization")

	if bearerToken == "" {
		return "", ErrNoAuthorizationHeader
	}

	if !strings.HasPrefix(bearerToken, "Bearer ") {
		return "", ErrMalformedAuthHeader
	}

	token := strings.TrimPrefix(bearerToken, "Bearer ")
	token = strings.TrimSpace(token) // clean up any accidental space

	if token == "" {
		return "", ErrNoTokenInAuthHeader
	}

	return token, nil
}

func GetUserFromContext(ctx context.Context) *mongodb.UserDb {
	if user, ok := ctx.Value(UserKey).(mongodb.UserDb); ok {
		return &user
	}
	return nil
}

func WithUser(ctx context.Context, user mongodb.UserDb) context.Context {
	return context.WithValue(ctx, UserKey, user)
}
