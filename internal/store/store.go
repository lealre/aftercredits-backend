package store

import (
	"context"

	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/models"
)

// Store is the storage-neutral persistence contract. Services and the api
// package depend on this interface (plus internal/models) instead of any
// concrete database implementation, so the underlying store can be swapped
// (e.g. Postgres) without touching business logic.
//
// Every method here is one that services/api actually call — grouped by entity
// below. Helpers internal to a particular implementation are intentionally
// excluded.
type Store interface {
	// ----- Users -----

	GetUserById(ctx context.Context, id string) (models.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, username, email string) (models.User, error)
	GetAllUsers(ctx context.Context) ([]models.User, error)
	UserExists(ctx context.Context, id string) (bool, error)
	AddUser(ctx context.Context, user models.User) error
	DeleteUserById(ctx context.Context, id string) error
	UpdateUserInfo(ctx context.Context, id string, user models.User) (models.User, error)
	UpdateUserLastLoginAt(ctx context.Context, userId string) (models.User, error)
	UpdateUserGroup(ctx context.Context, userId string, groupId string) (models.User, error)
	RemoveGroupFromUser(ctx context.Context, userId, groupId string) error

	// ----- Titles -----

	GetTitleById(ctx context.Context, id string) (models.Title, error)
	AddTitle(ctx context.Context, title models.Title) error
	DeleteTitle(ctx context.Context, id string) (bool, error)
	GetTitlesPage(ctx context.Context, orderBy string, ascending *bool, size, page int) ([]models.Title, int64, error)
	TitleExists(ctx context.Context, id string) (bool, error)

	// ----- Ratings -----
	//
	// A rating is a group-scoped fact keyed by (userId, titleId, groupId), so
	// every read that is not addressed by the rating's own id takes a groupId.
	// AddRating and UpdateRating carry it on the models.UserRating itself.

	AddRating(ctx context.Context, rating models.UserRating) (models.UserRating, error)
	GetRatingsByTitleId(ctx context.Context, titleId, groupId string) ([]models.UserRating, error)
	GetRatingById(ctx context.Context, ratingId, userId string) (models.UserRating, error)
	GetRatingByUserIdAndTitleId(ctx context.Context, userId, titleId, groupId string) (models.UserRating, error)
	UpdateRating(ctx context.Context, rating models.UserRating, userId string) (models.UserRating, error)
	GetRatingsByTitleIds(ctx context.Context, titleIds []string, groupId string) ([]models.UserRating, error)
	DeleteRating(ctx context.Context, ratingId, userId string) (int64, error)

	// ----- Comments -----
	//
	// Group-scoped on the same terms as ratings.

	GetCommentsByTitleId(ctx context.Context, titleId, groupId string) ([]models.Comment, error)
	GetUserCommentByTitleId(ctx context.Context, titleId, userId, groupId string) (models.Comment, error)
	GetCommentById(ctx context.Context, commentId string, userId string) (models.Comment, error)
	AddComment(ctx context.Context, comment models.Comment) (models.Comment, error)
	UpdateComment(ctx context.Context, comment models.Comment, userId string) (models.Comment, error)
	DeleteComment(ctx context.Context, commentId, userId, groupId string) (int64, error)

	// ----- Groups -----

	CreateGroup(ctx context.Context, group models.Group) (models.Group, error)
	GroupExists(ctx context.Context, groupId, userId string) (bool, error)
	GroupContainsTitle(ctx context.Context, groupId, titleId, userId string) (bool, error)
	GetGroupById(ctx context.Context, groupId, userId string) (models.Group, error)
	AddUserToGroup(ctx context.Context, groupId, ownerId, userToAddId string) error
	GetUsersFromGroup(ctx context.Context, groupId, userId string) ([]models.User, error)
	AddNewGroupTitle(ctx context.Context, groupId string, titleId string) error
	UpdateGroupTitleWatchedForMovie(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate) (*models.GroupTitleItem, error)
	UpdateGroupTitleWatchedForTVSeries(ctx context.Context, groupId string, titleId string, watched *bool, watchedAt *generics.FlexibleDate, season int, userId string) (*models.GroupTitleItem, error)
	UpdateGroupInfo(ctx context.Context, groupId, name, description string) error
	SoftDeleteGroup(ctx context.Context, groupId string) error
	RemoveUserFromGroup(ctx context.Context, groupId, userId string) error
	RemoveTitleFromGroup(ctx context.Context, groupId, titleId, userId string) error
	GetGroupTitlesPage(ctx context.Context, groupId string, watched *bool, titleTypes []string, orderBy string, ascending *bool, size, page int) ([]models.GroupPagedTitle, int64, error)
	GroupHasTitleEntries(ctx context.Context, groupId string, watched *bool, titleTypes []string) (bool, error)

	// ----- ActivityEvents -----

	InsertActivityEvents(ctx context.Context, events []models.ActivityEvent) error
	GetActivityFeed(ctx context.Context, userId string, before *int64, limit int) ([]models.ActivityEvent, error)
	GetActivityUnreadCount(ctx context.Context, userId string) (int64, error)
	MarkActivityRead(ctx context.Context, userId string, seq int64) error
}
