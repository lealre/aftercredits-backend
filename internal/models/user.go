package models

import "time"

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type User struct {
	Id           string
	Name         string
	Email        string
	Username     string
	PasswordHash string
	AvatarURL    *string
	Groups       []string
	Role         UserRole
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
