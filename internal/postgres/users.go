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
	groups, err := s.qq(ctx).GetUserGroupIds(ctx, row.ID)
	if err != nil {
		return models.User{}, err
	}
	return userRowToModel(row, groups), nil
}

func (s *Store) GetUserById(ctx context.Context, id string) (models.User, error) {
	row, err := s.qq(ctx).GetUserById(ctx, id)
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) GetUserByUsernameOrEmail(ctx context.Context, username, email string) (models.User, error) {
	row, err := s.qq(ctx).GetUserByUsernameOrEmail(ctx, database.GetUserByUsernameOrEmailParams{
		Username: username,
		Email:    email,
	})
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) GetAllUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.qq(ctx).GetAllUsers(ctx)
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
	return s.qq(ctx).UserExists(ctx, id)
}

func (s *Store) AddUser(ctx context.Context, user models.User) error {
	err := s.qq(ctx).CreateUser(ctx, database.CreateUserParams{
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

func (s *Store) DeleteUserById(ctx context.Context, id string) error {
	return s.qq(ctx).DeleteUserById(ctx, id)
}

func (s *Store) UpdateUserInfo(ctx context.Context, id string, user models.User) (models.User, error) {
	row, err := s.qq(ctx).UpdateUserInfo(ctx, database.UpdateUserInfoParams{
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
	row, err := s.qq(ctx).UpdateUserLastLoginAt(ctx, userId)
	if err != nil {
		return models.User{}, notFound(err)
	}
	return s.loadUser(ctx, row)
}

func (s *Store) UpdateUserGroup(ctx context.Context, userId string, groupId string) (models.User, error) {
	if err := s.qq(ctx).AddGroupMember(ctx, database.AddGroupMemberParams{
		GroupID: groupId,
		UserID:  userId,
	}); err != nil {
		return models.User{}, err
	}
	return s.GetUserById(ctx, userId)
}

func (s *Store) RemoveGroupFromUser(ctx context.Context, userId, groupId string) error {
	return s.qq(ctx).RemoveGroupMember(ctx, database.RemoveGroupMemberParams{
		GroupID: groupId,
		UserID:  userId,
	})
}
