package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var ErrCredentialsAlreadyExists = errors.New("username or email already exists")
var ErrInvalidEmail = errors.New("email format is not valid")
var ErrInvalidUsernameSize = errors.New("username must have at least 3 charscters")
var ErrInvalidUsername = errors.New("username must contains just letters, numbers, '-' or '_'")
var ErrInvalidPassword = errors.New("invalid passsword")

var ErrorMap = map[error]int{
	ErrInvalidUsername:          http.StatusBadRequest,
	ErrInvalidEmail:             http.StatusBadRequest,
	ErrInvalidUsernameSize:      http.StatusBadRequest,
	ErrInvalidPassword:          http.StatusBadRequest,
	ErrCredentialsAlreadyExists: http.StatusConflict,
}

func GetAllUsers(db *mongodb.DB, ctx context.Context) ([]UserResponse, error) {
	usersDb, err := db.GetAllUsers(ctx)
	if err != nil {
		return []UserResponse{}, err
	}

	var users []UserResponse
	for _, userDb := range usersDb {
		users = append(users, MapDbUserToApiUserResponse(userDb))
	}

	return users, nil
}

func GetUserById(db *mongodb.DB, ctx context.Context, id string) (UserResponse, error) {
	userDb, err := db.GetUserById(ctx, id)
	if err != nil {
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func AddUser(db *mongodb.DB, ctx context.Context, newUser NewUserRequest) (UserResponse, error) {
	if newUser.Email != "" && !IsValidEmail(newUser.Email) {
		return UserResponse{}, ErrInvalidEmail
	}

	if newUser.Username != "" {
		if len(newUser.Username) < 3 {
			return UserResponse{}, ErrInvalidUsernameSize
		}
		if !IsValidUsername(newUser.Username) {
			return UserResponse{}, ErrInvalidUsername
		}
	}

	if len(newUser.Password) < 4 {
		return UserResponse{}, ErrInvalidPassword
	}

	passorHash, err := auth.HashPassword(newUser.Password)
	if err != nil {
		return UserResponse{}, err
	}

	now := time.Now()
	userDb := mongodb.UserDb{
		Id:           primitive.NewObjectID().Hex(),
		Name:         newUser.Name,
		Username:     newUser.Username,
		Email:        newUser.Email,
		PasswordHash: passorHash,
		Role:         mongodb.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = db.AddUser(ctx, userDb)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return UserResponse{}, ErrCredentialsAlreadyExists
		}
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func DeleteUserById(db *mongodb.DB, ctx context.Context, id string) error {
	return db.DeleteUserById(ctx, id)
}
