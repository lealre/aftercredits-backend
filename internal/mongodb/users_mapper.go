package mongodb

import "github.com/lealre/movies-backend/internal/models"

// userDbToModel converts the mongo-specific UserDb into the storage-neutral
// models.User used by the service layer.
func userDbToModel(u UserDb) models.User {
	return models.User{
		Id:           u.Id,
		Name:         u.Name,
		Email:        u.Email,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		AvatarURL:    u.AvatarURL,
		Groups:       u.Groups,
		Role:         models.UserRole(u.Role),
		IsActive:     u.IsActive,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// userModelToDb converts a storage-neutral models.User back into the
// mongo-specific UserDb used at the persistence boundary.
func userModelToDb(u models.User) UserDb {
	return UserDb{
		Id:           u.Id,
		Name:         u.Name,
		Email:        u.Email,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		AvatarURL:    u.AvatarURL,
		Groups:       u.Groups,
		Role:         UserRole(u.Role),
		IsActive:     u.IsActive,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}
