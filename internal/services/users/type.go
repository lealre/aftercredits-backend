package users

import (
	"time"

	"github.com/lealre/movies-backend/internal/models"
)

type User struct {
	Id           string          `json:"id"`
	Name         string          `json:"name"`
	Email        string          `json:"email"`
	PasswordHash string          `json:"passwordHash"`
	AvatarURL    *string         `json:"avatarUrl,omitempty"`
	Groups       []string        `json:"groups,omitempty"`
	Role         models.UserRole `json:"role"`
	IsActive     bool            `json:"isActive"`
	LastLoginAt  *time.Time      `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type AllUsersResponse struct {
	Users []UserResponse `json:"users"`
}

// MemberResponse is the co-member view: what one group member may see about
// another. Deliberately narrower than UserResponse — no email, and no list of
// the other groups they belong to — so a single group membership does not leak
// every co-member's email address and cross-group id graph.
type MemberResponse struct {
	Id        string  `json:"id"`
	Username  string  `json:"username"`
	Name      string  `json:"name,omitempty"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}

// GroupMembersResponse keeps the historical {"users": [...]} envelope so
// existing clients keep parsing it; only each element is narrowed.
type GroupMembersResponse struct {
	Users []MemberResponse `json:"users"`
}

type NewUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	Id          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Name        string     `json:"name,omitempty"`
	AvatarURL   *string    `json:"avatarUrl,omitempty"`
	Groups      []string   `json:"groups,omitempty"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type UpdateUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// SetActiveRequest carries the admin's activate/deactivate intent. IsActive is
// a pointer so a missing field is distinguishable from an explicit false — a
// bare {} must be rejected, not silently read as "deactivate".
type SetActiveRequest struct {
	IsActive *bool `json:"isActive"`
}
