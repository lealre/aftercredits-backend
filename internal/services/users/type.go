package users

import (
	"time"

	"github.com/lealre/movies-backend/internal/mongodb"
)

type User struct {
	Id           string           `json:"id"`
	Name         string           `json:"name"`
	Email        string           `json:"email"`
	PasswordHash string           `json:"passwordHash"`
	AvatarURL    *string          `json:"avatarUrl,omitempty"`
	Groups       []string         `json:"groups,omitempty"`
	Role         mongodb.UserRole `json:"role"`
	IsActive     bool             `json:"isActive"`
	LastLoginAt  *time.Time       `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type AllUsersResponse struct {
	Users []UserResponse `json:"users"`
}

type NewUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
}

type UserResponse struct {
	Id          string     `json:"id"`
	Name        string     `json:"name"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	AvatarURL   *string    `json:"avatarUrl,omitempty"`
	Groups      []string   `json:"groups,omitempty"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
