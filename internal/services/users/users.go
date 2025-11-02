package users

import (
	"context"

	"github.com/lealre/movies-backend/internal/mongodb"
)

func GetAllUsers(db *mongodb.DB, ctx context.Context) ([]User, error) {
	usersDb, err := db.GetAllUsers(ctx)
	if err != nil {
		return []User{}, err
	}

	var users []User
	for _, userDb := range usersDb {
		users = append(users, MapDbUserToApiUser(userDb))
	}

	return users, nil
}

func GetUserById(db *mongodb.DB, ctx context.Context, id string) (User, error) {
	userDb, err := db.GetUserById(ctx, id)
	if err != nil {
		return User{}, err
	}

	return MapDbUserToApiUser(userDb), nil
}

func MapDbUserToApiUser(userDb mongodb.UserDb) User {
	return User(userDb)
}
