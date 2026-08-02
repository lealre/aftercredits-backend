package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// newTestUser builds a minimal, valid models.User for insertion, with a
// unique id/username/email so tests can run independently.
func newTestUser(t *testing.T) models.User {
	t.Helper()
	suffix := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)
	return models.User{
		Id:           "user-" + suffix,
		Name:         "Test User",
		Email:        "user-" + suffix + "@example.com",
		Username:     "user-" + suffix,
		PasswordHash: "hashed-password",
		Role:         models.RoleUser,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestStore_AddUser_GetUserById(t *testing.T) {
	t.Run("round trip without avatar", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		got, err := s.GetUserById(ctx, user.Id)
		require.NoError(t, err)
		require.Equal(t, user.Id, got.Id)
		require.Equal(t, user.Name, got.Name)
		require.Equal(t, user.Email, got.Email)
		require.Equal(t, user.Username, got.Username)
		require.Equal(t, user.PasswordHash, got.PasswordHash)
		require.Nil(t, got.AvatarURL, "avatar url should be nil when not set")
		require.Empty(t, got.Groups, "groups should be empty when user has no memberships")
		require.Equal(t, models.RoleUser, got.Role)
		require.True(t, got.IsActive)
		require.Nil(t, got.LastLoginAt)
		require.WithinDuration(t, user.CreatedAt, got.CreatedAt, time.Second)
		require.WithinDuration(t, user.UpdatedAt, got.UpdatedAt, time.Second)
	})

	t.Run("round trip with avatar and admin role", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		avatar := "https://example.com/avatar.png"
		user := newTestUser(t)
		user.AvatarURL = &avatar
		user.Role = models.RoleAdmin
		require.NoError(t, s.AddUser(ctx, user))

		got, err := s.GetUserById(ctx, user.Id)
		require.NoError(t, err)
		require.NotNil(t, got.AvatarURL)
		require.Equal(t, avatar, *got.AvatarURL)
		require.Equal(t, models.RoleAdmin, got.Role)
	})
}

func TestStore_AddUser_Duplicate(t *testing.T) {
	t.Run("duplicate username", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		dup := newTestUser(t)
		dup.Username = user.Username
		err := s.AddUser(ctx, dup)
		require.ErrorIs(t, err, store.ErrDuplicatedRecord)
	})

	t.Run("duplicate email", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		dup := newTestUser(t)
		dup.Email = user.Email
		err := s.AddUser(ctx, dup)
		require.ErrorIs(t, err, store.ErrDuplicatedRecord)
	})
}

func TestStore_GetUserById_NotFound(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetUserById(ctx, "missing-user")
	require.ErrorIs(t, err, store.ErrRecordNotFound)
}

func TestStore_GetUserByUsernameOrEmail(t *testing.T) {
	t.Run("found by username", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		got, err := s.GetUserByUsernameOrEmail(ctx, user.Username, "")
		require.NoError(t, err)
		require.Equal(t, user.Id, got.Id)
	})

	t.Run("found by email", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		got, err := s.GetUserByUsernameOrEmail(ctx, "", user.Email)
		require.NoError(t, err)
		require.Equal(t, user.Id, got.Id)
	})

	t.Run("not found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		_, err := s.GetUserByUsernameOrEmail(ctx, "nobody", "nobody@example.com")
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})

	t.Run("both supplied matching the same user is found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		user := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, user))

		got, err := s.GetUserByUsernameOrEmail(ctx, user.Username, user.Email)
		require.NoError(t, err)
		require.Equal(t, user.Id, got.Id)
	})

	// This is the case that distinguishes Mongo's AND semantics (a dynamic
	// filter that only requires the non-empty fields to match, on the SAME
	// document) from a plain OR: username matches user A while email matches
	// a completely different user B. Mongo's FindOne with both filter keys
	// set requires a single document satisfying both, so this must be
	// store.ErrRecordNotFound, not a match on either user.
	t.Run("both supplied but matching different users is not found", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		userA := newTestUser(t)
		userB := newTestUser(t)
		require.NoError(t, s.AddUser(ctx, userA))
		require.NoError(t, s.AddUser(ctx, userB))

		_, err := s.GetUserByUsernameOrEmail(ctx, userA.Username, userB.Email)
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})
}

func TestStore_UpdateUserInfo(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, s.AddUser(ctx, user))

	update := user
	update.Name = "Updated Name"
	update.Email = "updated-" + user.Email
	update.Username = "updated-" + user.Username

	got, err := s.UpdateUserInfo(ctx, user.Id, update)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", got.Name)
	require.Equal(t, update.Email, got.Email)
	require.Equal(t, update.Username, got.Username)
	require.True(t, got.UpdatedAt.After(user.UpdatedAt) || got.UpdatedAt.Equal(user.UpdatedAt))
}

func TestStore_UpdateUserLastLoginAt(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, s.AddUser(ctx, user))
	require.Nil(t, user.LastLoginAt)

	got, err := s.UpdateUserLastLoginAt(ctx, user.Id)
	require.NoError(t, err)
	require.NotNil(t, got.LastLoginAt)
	require.WithinDuration(t, time.Now(), *got.LastLoginAt, 5*time.Second)
}

func TestStore_UpdateUserGroup_RemoveGroupFromUser(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, s.AddUser(ctx, user))

	group := createTestGroup(t, s, user.Id)

	got, err := s.UpdateUserGroup(ctx, user.Id, group)
	require.NoError(t, err)
	require.Contains(t, got.Groups, group)

	reloaded, err := s.GetUserById(ctx, user.Id)
	require.NoError(t, err)
	require.Contains(t, reloaded.Groups, group)

	require.NoError(t, s.RemoveGroupFromUser(ctx, user.Id, group))

	afterRemoval, err := s.GetUserById(ctx, user.Id)
	require.NoError(t, err)
	require.NotContains(t, afterRemoval.Groups, group)
}

// createTestGroup inserts a minimal groups row owned by ownerId directly via
// the pool (postgres.Store has no group methods yet), returning the new
// group's id, so UpdateUserGroup/RemoveGroupFromUser can be exercised
// against a real, non-deleted group row.
func createTestGroup(t *testing.T, s *Store, ownerId string) string {
	t.Helper()
	id := "group-" + uuid.NewString()
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO groups (id, name, owner_id) VALUES ($1, $2, $3)`,
		id, "Test Group", ownerId,
	)
	require.NoError(t, err)
	return id
}

func TestStore_GetAllUsers(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	all, err := s.GetAllUsers(ctx)
	require.NoError(t, err)
	require.Empty(t, all)

	user1 := newTestUser(t)
	user2 := newTestUser(t)
	require.NoError(t, s.AddUser(ctx, user1))
	require.NoError(t, s.AddUser(ctx, user2))

	all, err = s.GetAllUsers(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)

	ids := []string{all[0].Id, all[1].Id}
	require.ElementsMatch(t, []string{user1.Id, user2.Id}, ids)
}

func TestStore_UserExists(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t)

	exists, err := s.UserExists(ctx, user.Id)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, s.AddUser(ctx, user))

	exists, err = s.UserExists(ctx, user.Id)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestStore_DeleteUserById(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, s.AddUser(ctx, user))

	require.NoError(t, s.DeleteUserById(ctx, user.Id))

	_, err := s.GetUserById(ctx, user.Id)
	require.ErrorIs(t, err, store.ErrRecordNotFound)
}
