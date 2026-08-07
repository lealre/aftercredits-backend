package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

func GetAllUsers(db store.Store, ctx context.Context) ([]UserResponse, error) {
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

func GetUserDbByUsernameOrEmail(db store.Store, ctx context.Context, username, email string) (models.User, error) {
	userDb, err := db.GetUserByUsernameOrEmail(ctx, username, email)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, err
	}

	return userDb, nil
}

func GetUserById(db store.Store, ctx context.Context, id string) (UserResponse, error) {
	userDb, err := db.GetUserById(ctx, id)
	if err != nil {
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func AddUser(db store.Store, ctx context.Context, newUser NewUserRequest) (UserResponse, error) {
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
	userDb := models.User{
		Id:           uuid.NewString(),
		Name:         newUser.Name,
		Username:     newUser.Username,
		Email:        newUser.Email,
		PasswordHash: passorHash,
		Role:         models.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = db.AddUser(ctx, userDb)
	if err != nil {
		if errors.Is(err, store.ErrDuplicatedRecord) {
			return UserResponse{}, ErrCredentialsAlreadyExists
		}
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func UpdateUserInfo(db store.Store, ctx context.Context, userId string, userUpdate UpdateUserRequest) (UserResponse, error) {
	newEmail := strings.TrimSpace(userUpdate.Email)
	newUsername := strings.TrimSpace(userUpdate.Username)
	newName := strings.TrimSpace(userUpdate.Name)

	userToUpdateDb, err := db.GetUserById(ctx, userId)
	if err != nil {
		return UserResponse{}, err
	}

	if newEmail != "" {
		if !IsValidEmail(newEmail) {
			return UserResponse{}, ErrInvalidEmail
		}
		userToUpdateDb.Email = newEmail
	}

	if newUsername != "" {
		if len(newUsername) < 3 {
			return UserResponse{}, ErrInvalidUsernameSize
		}
		if !IsValidUsername(newUsername) {
			return UserResponse{}, ErrInvalidUsername
		}
		userToUpdateDb.Username = newUsername
	}

	if newName != "" {
		userToUpdateDb.Name = newName
	}

	userUpdatedDb, err := db.UpdateUserInfo(ctx, userId, userToUpdateDb)
	if err != nil {
		if errors.Is(err, store.ErrDuplicatedRecord) {
			return UserResponse{}, ErrCredentialsAlreadyExists
		}
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userUpdatedDb), nil
}

func DeleteUserById(db store.Store, ctx context.Context, id string) error {
	return db.DeleteUserById(ctx, id)
}

func UpdateUserLastLoginAt(db store.Store, ctx context.Context, userId string) (UserResponse, error) {
	userDb, err := db.UpdateUserLastLoginAt(ctx, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func BuildLoginResponse(db store.Store, ctx context.Context, user models.User, token string) (auth.LoginResponse, error) {
	userResponse, err := UpdateUserLastLoginAt(db, ctx, user.Id)
	if err != nil {
		return auth.LoginResponse{}, err
	}
	return MapDbUserToApiLoginResponse(userResponse, token), nil
}

func UpdateUserGroup(db store.Store, ctx context.Context, userId string, groupId string) (UserResponse, error) {
	userDb, err := db.UpdateUserGroup(ctx, userId, groupId)

	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

// UserExists reports whether a user with the given id exists. Thin service
// passthrough so handlers reach the DB only through the service layer.
func UserExists(db store.Store, ctx context.Context, id string) (bool, error) {
	return db.UserExists(ctx, id)
}
