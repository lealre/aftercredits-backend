package auth

import "time"

type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email" `
	Password string `json:"password"`
}

type LoginResponse struct {
	Id          string     `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	Name        string     `json:"name,omitempty"`
	AvatarURL   *string    `json:"avatarUrl,omitempty"`
	Groups      []string   `json:"groups"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	AccessToken string     `json:"accessToken"`
}
