package users

import "time"

type User struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"passwordHash"`
	AvatarURL    *string    `json:"avatarUrl,omitempty"`
	Groups       []string   `json:"groups,omitempty"`
	Role         string     `json:"role"`
	IsActive     bool       `json:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type AllUsersResponse struct {
	Users []User `json:"users"`
}
