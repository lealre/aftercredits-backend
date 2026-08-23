package postgres

import (
	"context"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// loadUser resolves a database.User row's group ids and maps the pair into
// a models.User.
func (s *Store) loadUser(ctx context.Context, row database.User) (models.User, error) {
	groups, err := s.q.GetUserGroupIds(ctx, row.ID)
	if err != nil {
		return models.User{}, err
	}
	return userRowToModel(row, groups), nil
}

func (s *Store) GetUserById(ctx context.Context, id string) (models.User, error) {
	row, err := s.q.GetUserById(ctx, id)
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) GetUserByUsernameOrEmail(ctx context.Context, username, email string) (models.User, error) {
	row, err := s.q.GetUserByUsernameOrEmail(ctx, database.GetUserByUsernameOrEmailParams{
		Username: username,
		Email:    email,
	})
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) GetAllUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.q.GetAllUsers(ctx)
	if err != nil {
		return []models.User{}, err
	}

	users := make([]models.User, 0, len(rows))
	for _, row := range rows {
		user, err := s.loadUser(ctx, row)
		if err != nil {
			return []models.User{}, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	return s.q.UserExists(ctx, id)
}

func (s *Store) UserExistsByUsernameOrEmail(ctx context.Context, username, email string) (bool, error) {
	return s.q.UserExistsByUsernameOrEmail(ctx, database.UserExistsByUsernameOrEmailParams{
		Username: username,
		Email:    email,
	})
}

func (s *Store) AdminExists(ctx context.Context) (bool, error) {
	return s.q.AdminExists(ctx)
}

func (s *Store) AddUser(ctx context.Context, user models.User) error {
	err := s.q.CreateUser(ctx, database.CreateUserParams{
		ID:           user.Id,
		Name:         user.Name,
		Email:        user.Email,
		Username:     user.Username,
		PasswordHash: user.PasswordHash,
		AvatarUrl:    ptrToText(user.AvatarURL),
		Role:         string(user.Role),
		IsActive:     user.IsActive,
		LastLoginAt:  ptrToTimestamptz(user.LastLoginAt),
		CreatedAt:    timeToTimestamptz(user.CreatedAt),
		UpdatedAt:    timeToTimestamptz(user.UpdatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrDuplicatedRecord
		}
		return err
	}
	return nil
}

// DeleteUserById soft-deletes (deactivates) the user, reporting an unknown id
// as store.ErrRecordNotFound so the handler can answer 404 rather than a silent
// 200.
func (s *Store) DeleteUserById(ctx context.Context, id string) error {
	rows, err := s.q.DeleteUserById(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

// UpdateUserPassword rewrites the hash and bumps token_version in one
// statement, invalidating every token minted before the change.
func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	rows, err := s.q.UpdateUserPassword(ctx, database.UpdateUserPasswordParams{
		ID:           id,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

// IncrementUserTokenVersion invalidates every outstanding token for the user
// without changing the password ("log out everywhere").
func (s *Store) IncrementUserTokenVersion(ctx context.Context, id string) error {
	rows, err := s.q.IncrementUserTokenVersion(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

// SetUserActive flips the account's active flag. Setting false revokes access
// on the next request and closes any open stream on its next reconnect.
func (s *Store) SetUserActive(ctx context.Context, id string, active bool) error {
	rows, err := s.q.SetUserActive(ctx, database.SetUserActiveParams{
		ID:       id,
		IsActive: active,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrRecordNotFound
	}
	return nil
}

func (s *Store) UpdateUserInfo(ctx context.Context, id string, user models.User) (models.User, error) {
	row, err := s.q.UpdateUserInfo(ctx, database.UpdateUserInfoParams{
		ID:       id,
		Name:     user.Name,
		Email:    user.Email,
		Username: user.Username,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return models.User{}, store.ErrDuplicatedRecord
		}
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) UpdateUserLastLoginAt(ctx context.Context, userId string) (models.User, error) {
	row, err := s.q.UpdateUserLastLoginAt(ctx, userId)
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) UpdateUserGroup(ctx context.Context, userId string, groupId string) (models.User, error) {
	if err := s.q.AddGroupMember(ctx, database.AddGroupMemberParams{
		GroupID: groupId,
		UserID:  userId,
	}); err != nil {
		return models.User{}, err
	}
	return s.GetUserById(ctx, userId)
}

func (s *Store) RemoveGroupFromUser(ctx context.Context, userId, groupId string) error {
	return s.q.RemoveGroupMember(ctx, database.RemoveGroupMemberParams{
		GroupID: groupId,
		UserID:  userId,
	})
}
