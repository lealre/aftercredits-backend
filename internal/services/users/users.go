package users

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
)

func GetAllUsers(ctx context.Context) ([]User, error) {
	return getAllUsersDb(ctx)
}

func GetUserById(ctx context.Context, id string) (User, error) {
	return getUserByIdDb(ctx, id)
}

func CheckIfUserExist(ctx context.Context, id string) (bool, error) {
	_, err := getUserByIdDb(ctx, id)
	if err == nil {
		return true, nil
	}
	if err == mongodb.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}
