package comments

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/titles"
)

/*
* Gets all comments from a title in a specific group, including all users that are in the group.
* Assumes that the caller already checked that:
* - The group exists
* - The title is in the group
* - The user is in the group
 */
func GetCommentsByTitleId(db *mongodb.DB, ctx context.Context, groupId, titleId, userId string) ([]Comment, error) {
	group, err := db.GetGroupById(ctx, groupId, userId)
	if err != nil {
		return []Comment{}, err
	}

	commentsDb, err := db.GetCommentsByTitleId(ctx, titleId, group.Users)
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
func AddComment(db *mongodb.DB, ctx context.Context, newComment NewComment, userId string, title titles.Title) (Comment, error) {
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

func addCommentForMovie(db *mongodb.DB, ctx context.Context, newComment NewComment, userId string) (Comment, error) {
	newCommentDb := mongodb.CommentDb{
		TitleId: newComment.TitleId,
		UserId:  userId,
		Comment: &newComment.Comment,
	}

	commentDb, err := db.AddComment(ctx, newCommentDb)
	if err != nil {
		if errors.Is(err, mongodb.ErrDuplicatedRecord) {
			return Comment{}, ErrCommentAlreadyExists
		}
		return Comment{}, err
	}

	return MapDbCommentToApiComment(commentDb), nil
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
func addCommentForTVSeries(db *mongodb.DB, ctx context.Context, newComment NewComment, userId string, title titles.Title) (Comment, error) {
	// 1. Validates that a season number is provided
	if newComment.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	// 2. Checks if a comment already exists for this user/title combination
	existingComment, err := db.GetUserCommentByTitleId(ctx, newComment.TitleId, userId)
	hasComment := err == nil
	if err != nil && err != mongodb.ErrRecordNotFound {
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

	var commentDb mongodb.CommentDb
	if !hasComment {
		// 4.1. Creates a new comment with the season comment
		newCommentDb := mongodb.CommentDb{
			TitleId: newComment.TitleId,
			UserId:  userId,
			Comment: nil,
			SeasonsComments: &mongodb.SeasonsCommentsDb{
				seasonAsString: newComment.Comment,
			},
		}

		commentDb, err = db.AddComment(ctx, newCommentDb)
		if err != nil {
			if errors.Is(err, mongodb.ErrDuplicatedRecord) {
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
			(*existingComment.SeasonsComments)[seasonAsString] = newComment.Comment
		} else {
			// 5.3. Adds the new season comment to the existing comment
			existingComment.SeasonsComments = &mongodb.SeasonsCommentsDb{
				seasonAsString: newComment.Comment,
			}
		}
		// 5.4. Updates the existing comment in the database
		commentDb, err = db.UpdateComment(ctx, existingComment, userId)
		if err != nil {
			return Comment{}, err
		}
	}

	return MapDbCommentToApiComment(commentDb), nil
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
func UpdateComment(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	logger := logx.FromContext(ctx)
	if strings.TrimSpace(updateReq.Comment) == "" {
		return Comment{}, ErrCommentIsNull
	}

	if updateReq.Season != nil && *updateReq.Season <= 0 {
		return Comment{}, ErrInvalidSeasonValue
	}

	if title.Type == "tvSeries" || title.Type == "tvMiniSeries" {
		logger.Printf("Updating comment for TV series %s", commentId)
		return updateCommentForTVSeries(db, ctx, commentId, userId, updateReq, title)
	}

	logger.Printf("Updating comment for movie %s", commentId)
	return updateCommentForMovie(db, ctx, commentId, userId, updateReq)

}

func updateCommentForMovie(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest) (Comment, error) {
	commentDb := mongodb.CommentDb{
		Id:      commentId,
		Comment: &updateReq.Comment,
	}
	updatedCommentDb, err := db.UpdateComment(ctx, commentDb, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}
	return MapDbCommentToApiComment(updatedCommentDb), nil
}

// updateCommentForTVSeries updates the comment of a specific season of a TV series.
//
// Steps performed by this method:
//  1. Validates that a season number is provided in the update request.
//  2. Fetches the existing comment for the given user and title.
//  3. Ensures that the existing comment has season comments.
//  4. Verifies that the specified season exists within the stored season comments.
//  5. Updates the comment for the specified season.
//  6. Persists the updated season comments to the database.
//
// Possible errors:
//   - ErrSeasonRequired: if no season is provided in the update request.
//   - ErrCommentNotFound: if the comment or the specified season comment does not exist.
//   - Any error returned by db.UpdateComment when persisting the update.
func updateCommentForTVSeries(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	// 1. Validate that a season number is provided in the update request
	if updateReq.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	// 2. Fetch the existing comment for this user and title
	existingComment, err := db.GetUserCommentByTitleId(ctx, title.Id, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}

	// 3. Ensure that the existing comment has season comments
	seasonAsString := strconv.Itoa(*updateReq.Season)

	if existingComment.SeasonsComments == nil {
		return Comment{}, ErrCommentNotFound
	}

	// 4. Verify that the specified season exists in the stored season comments
	if _, exists := (*existingComment.SeasonsComments)[seasonAsString]; !exists {
		return Comment{}, ErrCommentNotFound
	}

	// 5. Update the comment for the specified season
	(*existingComment.SeasonsComments)[seasonAsString] = updateReq.Comment

	// 6. Persist the updated season comments to the database
	converted := mongodb.SeasonsCommentsDb(*existingComment.SeasonsComments)
	seasonsComments := &converted

	commentDb := mongodb.CommentDb{
		Id:              commentId,
		SeasonsComments: seasonsComments,
	}

	updatedCommentDb, err := db.UpdateComment(ctx, commentDb, userId)
	if err != nil {
		return Comment{}, err
	}

	return MapDbCommentToApiComment(updatedCommentDb), nil
}

func DeleteComment(db *mongodb.DB, ctx context.Context, commentId, userId string) (int64, error) {
	deletedCount, err := db.DeleteComment(ctx, commentId, userId)
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
func DeleteCommentSeason(db *mongodb.DB, ctx context.Context, commentId, userId string, season int, title titles.Title) error {
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

	// Fetch the comment by id (and ensure it belongs to the user)
	existingComment, err := db.GetCommentById(ctx, commentId, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return ErrCommentNotFound
		}
		return err
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
		deleted, err := db.DeleteComment(ctx, commentId, userId)
		if err != nil {
			return err
		}
		if deleted == 0 {
			return ErrCommentNotFound
		}
		return nil
	}

	// Persist updated seasons map
	converted := mongodb.SeasonsCommentsDb(*existingComment.SeasonsComments)
	seasonsComments := &converted

	commentDb := mongodb.CommentDb{
		Id:              commentId,
		Comment:         existingComment.Comment,
		SeasonsComments: seasonsComments,
	}

	_, err = db.UpdateComment(ctx, commentDb, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return ErrCommentNotFound
		}
		return err
	}

	return nil
}
