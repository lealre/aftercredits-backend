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

func updateCommentForTVSeries(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest, title titles.Title) (Comment, error) {
	if updateReq.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	existingComment, err := db.GetUserCommentByTitleId(ctx, title.Id, userId)
	if err != nil {
		if err == mongodb.ErrRecordNotFound {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}

	seasonAsString := strconv.Itoa(*updateReq.Season)

	if existingComment.SeasonsComments == nil {
		return Comment{}, ErrCommentNotFound
	}

	if _, exists := (*existingComment.SeasonsComments)[seasonAsString]; !exists {
		return Comment{}, ErrCommentNotFound
	}
	(*existingComment.SeasonsComments)[seasonAsString] = updateReq.Comment

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
