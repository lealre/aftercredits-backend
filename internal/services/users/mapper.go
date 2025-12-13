package users

import (
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/mongodb"
)

func MapDbUserToApiUserResponse(userDb mongodb.UserDb) UserResponse {
	return UserResponse{
		Id:          userDb.Id,
		Username:    userDb.Username,
		Name:        userDb.Name,
		Email:       userDb.Email,
		CreatedAt:   userDb.CreatedAt,
		UpdatedAt:   userDb.UpdatedAt,
		LastLoginAt: userDb.LastLoginAt,
		AvatarURL:   userDb.AvatarURL,
		Groups:      userDb.Groups,
	}
}

func MapDbUserToApiLoginResponse(userResponse UserResponse, token string) auth.LoginResponse {
	return auth.LoginResponse{
		Id:          userResponse.Id,
		Email:       userResponse.Email,
		Username:    userResponse.Username,
		Name:        userResponse.Name,
		AvatarURL:   userResponse.AvatarURL,
		Groups:      userResponse.Groups,
		LastLoginAt: userResponse.LastLoginAt,
		AccessToken: token,
	}
}
