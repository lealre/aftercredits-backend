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

func addCommentForTVSeries(db *mongodb.DB, ctx context.Context, newComment NewComment, userId string, title titles.Title) (Comment, error) {
	if newComment.Season == nil {
		return Comment{}, ErrSeasonRequired
	}

	// check is the comment already exists for this user/title combination
	existingComment, err := db.GetUserCommentByTitleId(ctx, newComment.TitleId, userId)
	hasComment := err == nil
	if err != nil && err != mongodb.ErrRecordNotFound {
		return Comment{}, err
	}

	// Check if the season exists in the title
	seasonAsString := strconv.Itoa(*newComment.Season)
	seasonExists := false
	for _, season := range title.Seasons {
		if season.Season == seasonAsString {
			seasonExists = true
			break
		}
	}
	// If season does not exist for title, return error
	if !seasonExists {
		return Comment{}, ErrSeasonDoesNotExist
	}

	var commentDb mongodb.CommentDb
	if !hasComment {
		// Create new comment for season
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
		if existingComment.SeasonsComments != nil {
			if _, exists := (*existingComment.SeasonsComments)[seasonAsString]; exists {
				return Comment{}, ErrSeasonCommentAlreadyExists
			}
			(*existingComment.SeasonsComments)[seasonAsString] = newComment.Comment
		} else {
			existingComment.SeasonsComments = &mongodb.SeasonsCommentsDb{
				seasonAsString: newComment.Comment,
			}
		}
		commentDb, err = db.UpdateComment(ctx, existingComment, userId)
		if err != nil {
			return Comment{}, err
		}
	}

	return MapDbCommentToApiComment(commentDb), nil
}

func UpdateComment(db *mongodb.DB, ctx context.Context, commentId, userId string, updateReq UpdateCommentRequest) (Comment, error) {
	if strings.TrimSpace(updateReq.Comment) == "" {
		return Comment{}, ErrCommentIsNull
	}

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

func DeleteComment(db *mongodb.DB, ctx context.Context, commentId, userId string) (int64, error) {
	deletedCount, err := db.DeleteComment(ctx, commentId, userId)
	if err != nil {
		return 0, err
	}

	return deletedCount, nil
}
