package users

import "github.com/lealre/movies-backend/internal/mongodb"

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
