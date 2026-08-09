package comments

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/store"
)

/*
* Gets all comments left on a title inside a specific group.
*
* A comment is a group-scoped fact, so the store filters on group_id directly;
* there is no member-list filter to assemble. Authorization stays with the
* caller: the handler's groups.GroupContainsTitle guard already establishes
* that the group exists, is not deleted, holds the title and has the caller as
* a member — strictly more than this read needs, so repeating any part of it
* here would be a second membership query that can never answer differently.
 */
func GetCommentsByTitleId(db store.Store, ctx context.Context, groupId, titleId string) ([]Comment, error) {
	commentsDb, err := db.GetCommentsByTitleId(ctx, titleId, groupId)
	if err != nil {
		return []Comment{}, err
	}

	var comments []Comment
	for _, commentDb := range commentsDb {
		comments = append(comments, MapDbCommentToApiComment(commentDb))
	}

	return comments, nil
}

// AddComment creates a new comment for a title.
//
// Routes to the appropriate handler based on title type (TV series or movie):
//   - addCommentForTVSeries: If the title is a TV series (tvSeries or tvMiniSeries)
//   - addCommentForMovie: If the title is a movie (non-TV series)
func AddComment(db store.Store, ctx context.Context, newComment NewComment, userId string, title titles.Title) (Comment, error) {
	logger := logx.FromContext(ctx)
	if strings.TrimSpace(newComment.Comment) == "" {
		return Comment{}, ErrCommentIsNull
	}

	if newComment.Season != nil && *newComment.Season <= 0 {
		return Comment{}, ErrInvalidSeasonValue
	}

	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Adding comment for TV series %s", newComment.TitleId)
		return addCommentForTVSeries(db, ctx, newComment, userId, title)
	}

	logger.Printf("Adding comment for movie %s", newComment.TitleId)
	return addCommentForMovie(db, ctx, newComment, userId)
}

func addCommentForMovie(db store.Store, ctx context.Context, newComment NewComment, userId string) (Comment, error) {
	newCommentModel := models.Comment{
		TitleId: newComment.TitleId,
		UserId:  userId,
		GroupId: newComment.GroupId,
		Comment: &newComment.Comment,
	}

	comment, err := db.AddComment(ctx, newCommentModel)
	if err != nil {
		if errors.Is(err, store.ErrDuplicatedRecord) {
			return Comment{}, ErrCommentAlreadyExists
		}
		return Comment{}, err
	}

	return MapDbCommentToApiComment(comment), nil
}

// addCommentForTVSeries handles comment creation/update for TV series (tvSeries or tvMiniSeries).
//
//  1. Validates that a season number is provided in the comment request
//  2. Checks if a comment already exists for this user/title combination
//  3. Validates that the season exists in the title's seasons list
//  4. If no comment exists:
//     4.1. Creates a new comment with the season comment
//  5. If a comment exists:
//     5.1. Checks if a comment for this specific season already exists
//     5.2. If the season comment exists: Returns ErrSeasonCommentAlreadyExists
//     5.3. If the season comment doesn't exist: Adds the new season comment to the existing comment
//     5.4. Updates the existing comment in the database
//
// Returns:
//   - Comment: The created or updated comment with all fields populated
//   - error: Returns various errors based on validation failures:
//   - ErrSeasonRequired: If season is missing
//   - ErrSeasonDoesNotExist: If the season doesn't exist in the title
//   - ErrSeasonCommentAlreadyExists: If comment for this season already exists
func addCommentForTVSeries(db store.Store, ctx context.Context, newComment NewComment, userId string, title titles.Title) (Comment, error) {
	// 1. Validates that a season number is provided
	if newComment.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	// 2. Checks if a comment already exists for this user/title/group combination.
	// The lookup is group-scoped: a comment the same user left on this title in
	// another group is a different fact and must not be picked up here.
	existingComment, err := db.GetUserCommentByTitleId(ctx, newComment.TitleId, userId, newComment.GroupId)
	hasComment := err == nil
	if err != nil && err != store.ErrRecordNotFound {
		return Comment{}, err
	}

	seasonAsString := strconv.Itoa(*newComment.Season)

	// 3. Validates that the season exists in the title's seasons list
	seasonExists := false
	for _, season := range title.Seasons {
		if season.Season == seasonAsString {
			seasonExists = true
			break
		}
	}
	if !seasonExists {
		return Comment{}, ErrSeasonDoesNotExist
	}

	var comment models.Comment
	now := time.Now()
	if !hasComment {
		// 4.1. Creates a new comment with the season comment
		newCommentModel := models.Comment{
			TitleId: newComment.TitleId,
			UserId:  userId,
			GroupId: newComment.GroupId,
			Comment: nil,
			SeasonsComments: &models.SeasonsComments{
				seasonAsString: models.SeasonCommentItem{
					Comment:   newComment.Comment,
					AddedAt:   now,
					UpdatedAt: now,
				},
			},
		}

		comment, err = db.AddComment(ctx, newCommentModel)
		if err != nil {
			if errors.Is(err, store.ErrDuplicatedRecord) {
				return Comment{}, ErrSeasonCommentAlreadyExists
			}
			return Comment{}, err
		}

	} else {
		// 5.1. Checks if a comment for this specific season already exists
		if existingComment.SeasonsComments != nil {
			if _, exists := (*existingComment.SeasonsComments)[seasonAsString]; exists {
				// 5.2. Returns ErrSeasonCommentAlreadyExists
				return Comment{}, ErrSeasonCommentAlreadyExists
			}
			// 5.3. Adds the new season comment to the existing comment
			(*existingComment.SeasonsComments)[seasonAsString] = models.SeasonCommentItem{
				Comment:   newComment.Comment,
				AddedAt:   now,
				UpdatedAt: now,
			}
		} else {
			// 5.3. Adds the new season comment to the existing comment
			existingComment.SeasonsComments = &models.SeasonsComments{
				seasonAsString: models.SeasonCommentItem{
					Comment:   newComment.Comment,
					AddedAt:   now,
					UpdatedAt: now,
				},
			}
		}
		// 5.4. Updates the existing comment in the database
		comment, err = db.UpdateComment(ctx, existingComment, userId)
		if err != nil {
			return Comment{}, err
		}
	}

	return MapDbCommentToApiComment(comment), nil
}

// UpdateComment updates an existing comment for a given title.
//
// It performs basic validations on the incoming request and then delegates to
// the appropriate handler based on the title type:
//   - updateCommentForTVSeries: if the title is a TV series (tvSeries or tvMiniSeries)
//   - updateCommentForMovie: if the title is a movie (non‑TV series)
//
// Possible errors:
//   - ErrCommentIsNull: if the updated comment text is empty or whitespace
//   - ErrInvalidSeasonValue: if a season is provided and is less than or equal to zero
//   - ErrCommentNotFound: if the underlying specific handler cannot find the target comment
//   - Any error propagated from the underlying database operations
func UpdateComment(db store.Store, ctx context.Context, groupId, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	logger := logx.FromContext(ctx)
	if strings.TrimSpace(updateReq.Comment) == "" {
		return Comment{}, ErrCommentIsNull
	}

	if updateReq.Season != nil && *updateReq.Season <= 0 {
		return Comment{}, ErrInvalidSeasonValue
	}

	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Updating comment for TV series %s", commentId)
		return updateCommentForTVSeries(db, ctx, groupId, commentId, userId, updateReq, title)
	}

	logger.Printf("Updating comment for movie %s", commentId)
	return updateCommentForMovie(db, ctx, groupId, commentId, userId, updateReq, title)

}

// updateCommentForMovie updates the comment the caller left on a movie inside
// groupId.
//
// The row to write is resolved by (title, user, group) and the path's commentId
// is only accepted when it names that very row. The caller may hold a comment on
// the same movie in another group, and a client that resolves comment ids from a
// stale or unfiltered list would otherwise address one group's URL with another
// group's id — updating by id alone would then edit a group the request never
// named.
func updateCommentForMovie(db store.Store, ctx context.Context, groupId, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	existingComment, err := db.GetUserCommentByTitleId(ctx, title.Id, userId, groupId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}

	if existingComment.Id != commentId {
		return Comment{}, ErrCommentNotFound
	}

	comment := models.Comment{
		Id:      existingComment.Id,
		Comment: &updateReq.Comment,
	}
	updatedComment, err := db.UpdateComment(ctx, comment, userId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}
	return MapDbCommentToApiComment(updatedComment), nil
}

// updateCommentForTVSeries updates the comment of a specific season of a TV series.
//
// Steps performed by this method:
//  1. Validates that a season number is provided in the update request.
//  2. Fetches the existing comment for the given user, title and group, and
//     rejects a commentId that names a different row.
//  3. Ensures that the existing comment has season comments.
//  4. Verifies that the specified season exists within the stored season comments.
//  5. Updates the comment for the specified season (preserve AddedAt, update UpdatedAt).
//  6. Persists the updated season comments to the database.
//
// Possible errors:
//   - ErrSeasonRequired: if no season is provided in the update request.
//   - ErrCommentNotFound: if the comment or the specified season comment does not exist,
//     or if commentId belongs to a comment outside groupId.
//   - Any error returned by db.GetUserCommentByTitleId or db.UpdateComment when
//     fetching or persisting the update.
func updateCommentForTVSeries(db store.Store, ctx context.Context, groupId, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	// 1. Validate that a season number is provided in the update request
	if updateReq.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	// 2. Fetch the existing comment for this user, title and group. Without the
	// group the (user, title) lookup would match one row per group and silently
	// resolve to an arbitrary one. The row this resolves to is also the row that
	// gets written: the path's commentId is accepted only when it names it, so a
	// PATCH under group A's URL can never reach the caller's group B comment.
	existingComment, err := db.GetUserCommentByTitleId(ctx, title.Id, userId, groupId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}

	if existingComment.Id != commentId {
		return Comment{}, ErrCommentNotFound
	}

	// 3. Ensure that the existing comment has season comments
	seasonAsString := strconv.Itoa(*updateReq.Season)

	if existingComment.SeasonsComments == nil {
		return Comment{}, ErrCommentNotFound
	}

	// 4. Verify that the specified season exists in the stored season comments
	existingSeasonComment, exists := (*existingComment.SeasonsComments)[seasonAsString]
	if !exists {
		return Comment{}, ErrCommentNotFound
	}

	// 5. Update the comment for the specified season (preserve AddedAt, update
	// UpdatedAt). existingComment is the stored row with every season's
	// timestamps already assembled, so there is nothing to re-read.
	now := time.Now()
	seasonsComments := existingComment.SeasonsComments

	// Update only the season being modified
	(*seasonsComments)[seasonAsString] = models.SeasonCommentItem{
		Comment:   updateReq.Comment,
		AddedAt:   existingSeasonComment.AddedAt,
		UpdatedAt: now,
	}

	// 6. Persist the updated season comments to the database
	comment := models.Comment{
		Id:              existingComment.Id,
		SeasonsComments: seasonsComments,
	}

	updatedComment, err := db.UpdateComment(ctx, comment, userId)
	if err != nil {
		return Comment{}, err
	}

	return MapDbCommentToApiComment(updatedComment), nil
}

// DeleteComment deletes the caller's comment inside groupId.
//
// The route is group-scoped, so the delete is too: groupId is part of the key
// the store deletes on, and a commentId naming the caller's comment in another
// group matches nothing and reports 0 rows deleted rather than destroying a
// group the request never addressed.
func DeleteComment(db store.Store, ctx context.Context, commentId, userId, groupId string) (int64, error) {
	deletedCount, err := db.DeleteComment(ctx, commentId, userId, groupId)
	if err != nil {
		return 0, err
	}

	return deletedCount, nil
}

// DeleteCommentSeason deletes a comment for a specific season of a TV series.
//
// It follows the same TV-series season validation logic as addCommentForTVSeries:
//   - season must be > 0
//   - season must exist in the title's seasons list
//   - the season comment must exist in the stored seasonsComments map
//
// If, after deleting the season entry, there are no seasons left, the whole comment document is deleted.
//
// Like the other group-scoped comment routes, the row is resolved by
// (title, user, group) and the path's commentId is accepted only when it names
// that row, so a DELETE under one group's URL can never reach the caller's
// comment in another group.
func DeleteCommentSeason(db store.Store, ctx context.Context, groupId, commentId, userId string, season int, title titles.Title) error {
	if season <= 0 {
		return ErrInvalidSeasonValue
	}

	// This endpoint is specific for TV series season comments
	if title.Type != "tvSeries" && title.Type != "tvMiniSeries" {
		return ErrSeasonDoesNotExist
	}

	seasonAsString := strconv.Itoa(season)

	// Validate that the season exists in the title's seasons list
	seasonExists := false
	for _, s := range title.Seasons {
		if s.Season == seasonAsString {
			seasonExists = true
			break
		}
	}
	if !seasonExists {
		return ErrSeasonDoesNotExist
	}

	// Fetch the comment by (title, user, group), and accept the path's commentId
	// only when it names that row
	existingComment, err := db.GetUserCommentByTitleId(ctx, title.Id, userId, groupId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			return ErrCommentNotFound
		}
		return err
	}

	if existingComment.Id != commentId {
		return ErrCommentNotFound
	}

	// Ensure it has season comments and the requested season exists
	if existingComment.SeasonsComments == nil {
		return ErrCommentNotFound
	}
	if _, ok := (*existingComment.SeasonsComments)[seasonAsString]; !ok {
		return ErrCommentNotFound
	}

	// Delete season entry
	delete(*existingComment.SeasonsComments, seasonAsString)

	// If no seasons left, delete the whole comment document
	if len(*existingComment.SeasonsComments) == 0 {
		deleted, err := db.DeleteComment(ctx, existingComment.Id, userId, groupId)
		if err != nil {
			return err
		}
		if deleted == 0 {
			return ErrCommentNotFound
		}
		return nil
	}

	// Persist updated seasons map
	converted := models.SeasonsComments(*existingComment.SeasonsComments)
	seasonsComments := &converted

	comment := models.Comment{
		Id:              existingComment.Id,
		Comment:         existingComment.Comment,
		SeasonsComments: seasonsComments,
	}

	_, err = db.UpdateComment(ctx, comment, userId)
	if err != nil {
		if err == store.ErrRecordNotFound {
			return ErrCommentNotFound
		}
		return err
	}

	return nil
}
