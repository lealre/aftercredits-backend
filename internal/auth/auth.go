package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lealre/movies-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserKey contextKey = "user"

// tokenIssuer is written into every token and verified on every parse. A fixed
// issuer lets ValidateJWT reject a token minted for some other service that
// happens to share the signing secret.
const tokenIssuer = "mytitles"

// Claims is the token payload. It embeds the standard registered claims and
// adds token_version: the value the user row held when the token was minted.
// AuthMiddleware compares it against the row's current version, so bumping the
// column (password change, "log out everywhere") invalidates every token
// already in the wild.
type Claims struct {
	jwt.RegisteredClaims
	TokenVersion int `json:"tv"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// dummyHash is a real bcrypt hash at DefaultCost. FakePasswordCheck compares
// against it to burn the same CPU a genuine password check would, so the login
// path for a non-existent account takes the same time as the wrong-password
// path and cannot be used as a timing oracle to enumerate accounts.
const dummyHash = "$2a$10$OoIoDniQZbnWPq1Qcs4ixewQzkBw0FVp0H5zT1NvB4yxgWq/dLpyS"

// FakePasswordCheck performs a throwaway bcrypt comparison. Its result is
// discarded; it exists only for its timing side effect.
func FakePasswordCheck(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}

func MakeJWT(userID string, tokenVersion int, tokenSecret string, expiresIn time.Duration) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			Subject:   userID,
		},
		TokenVersion: tokenVersion,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return string(signedToken), nil
}

// ValidateJWT verifies the signature and standard claims and returns the
// subject and the token's version. The parser is pinned to HS256 specifically
// (not just the HMAC family), requires an expiry to be present, and requires
// the expected issuer — so an alg=none token, an unexpired-forever token, or a
// token minted for another service is rejected before any claim is trusted.
func ValidateJWT(tokenString, tokenSecret string) (string, int, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrTokenSigningMethod
			}
			return []byte(tokenSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(tokenIssuer),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", 0, ErrTokenExpired
		}
		return "", 0, ErrInvalidToken
	}

	if claims.Subject == "" {
		return "", 0, ErrTokenWithNoSubject
	}

	return claims.Subject, claims.TokenVersion, nil
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

func GetUserFromContext(ctx context.Context) *models.User {
	if user, ok := ctx.Value(UserKey).(models.User); ok {
		return &user
	}
	return nil
}

func WithUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}
