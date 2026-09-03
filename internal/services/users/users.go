package users

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/validate"
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
	if err := validateUserStrings(newUser.Name, newUser.Email, newUser.Username); err != nil {
		return UserResponse{}, err
	}

	if err := validatePassword(newUser.Password); err != nil {
		return UserResponse{}, err
	}

	// Reject a duplicate BEFORE hashing, so a replayed registration does not
	// cost a full bcrypt before the collision is detected. The unique indexes
	// remain the race backstop (handled below).
	exists, err := db.UserExistsByUsernameOrEmail(ctx, newUser.Username, newUser.Email)
	if err != nil {
		return UserResponse{}, err
	}
	if exists {
		return UserResponse{}, ErrCredentialsAlreadyExists
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
		if validate.TooLong(newEmail, validate.EmailMax) {
			return UserResponse{}, ErrInvalidEmailSize
		}
		if !IsValidEmail(newEmail) {
			return UserResponse{}, ErrInvalidEmail
		}
		userToUpdateDb.Email = newEmail
	}

	if newUsername != "" {
		if utf8.RuneCountInString(newUsername) < validate.UsernameMin || validate.TooLong(newUsername, validate.UsernameMax) {
			return UserResponse{}, ErrInvalidUsernameSize
		}
		if !IsValidUsername(newUsername) {
			return UserResponse{}, ErrInvalidUsername
		}
		userToUpdateDb.Username = newUsername
	}

	if newName != "" {
		if validate.TooLong(newName, validate.NameMax) || validate.HasControlChars(newName) {
			return UserResponse{}, ErrInvalidNameSize
		}
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
	err := db.DeleteUserById(ctx, id)
	if errors.Is(err, store.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

// ChangePassword verifies the caller's current password, enforces the password
// policy on the new one, and rewrites the hash — which also bumps the user's
// token_version, logging out every other session.
func ChangePassword(db store.Store, ctx context.Context, userId, currentPassword, newPassword string) error {
	userDb, err := db.GetUserById(ctx, userId)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	if err := auth.CheckPasswordHash(userDb.PasswordHash, currentPassword); err != nil {
		return ErrInvalidCurrentPassword
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := db.UpdateUserPassword(ctx, userId, newHash); err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// LogoutEverywhere invalidates every outstanding token for the user by bumping
// token_version, without changing the password.
func LogoutEverywhere(db store.Store, ctx context.Context, userId string) error {
	err := db.IncrementUserTokenVersion(ctx, userId)
	if errors.Is(err, store.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

// SetUserActive is the admin kill switch / reinstate.
func SetUserActive(db store.Store, ctx context.Context, targetUserId string, active bool) error {
	err := db.SetUserActive(ctx, targetUserId, active)
	if errors.Is(err, store.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
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
