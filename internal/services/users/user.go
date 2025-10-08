package users

import "context"

// CheckIfUserExist returns true when a user with the provided id exists.
// It returns false and nil error when the user does not exist.
// For other database errors, it returns false with the error for callers to handle.
func CheckIfUserExist(ctx context.Context, id string) (bool, error) {
	_, err := getUserByID(ctx, id)
	if err == nil {
		return true, nil
	}
	if err == ErrUserNotFound {
		return false, nil
	}
	return false, err
}
