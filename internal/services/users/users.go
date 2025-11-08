package users

import (
	"context"
	"time"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	passorHash, err := auth.HashPassword(newUser.Password)
	if err != nil {
		return UserResponse{}, err
	}

	now := time.Now()
	userDb := mongodb.UserDb{
		Id:           primitive.NewObjectID().Hex(),
		Name:         newUser.Name,
		Email:        newUser.Email,
		PasswordHash: passorHash,
		Role:         mongodb.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = db.AddUser(ctx, userDb)
	if err != nil {
		return UserResponse{}, err
	}

	return MapDbUserToApiUserResponse(userDb), nil
}

func MapDbUserToApiUserResponse(userDb mongodb.UserDb) UserResponse {
	return UserResponse{
		Id:          userDb.Id,
		Name:        userDb.Name,
		Email:       userDb.Email,
		CreatedAt:   userDb.CreatedAt,
		UpdatedAt:   userDb.UpdatedAt,
		LastLoginAt: userDb.LastLoginAt,
		AvatarURL:   userDb.AvatarURL,
		Groups:      userDb.Groups,
	}
}
