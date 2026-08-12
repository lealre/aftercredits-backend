package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// insertCommentSeasons inserts every entry of seasons (if any) as a
// comment_seasons row belonging to commentId, using qtx (a possibly
// tx-bound *database.Queries).
func insertCommentSeasons(ctx context.Context, qtx *database.Queries, commentId string, seasons *models.SeasonsComments) error {
	if seasons == nil {
		return nil
	}
	for season, item := range *seasons {
		if err := qtx.InsertCommentSeason(ctx, database.InsertCommentSeasonParams{
			CommentID: commentId,
			Season:    season,
			Comment:   item.Comment,
			AddedAt:   timeToTimestamptz(item.AddedAt),
			UpdatedAt: timeToTimestamptz(item.UpdatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

// assembleCommentRows builds a models.Comment for each row, fetching all of
// their comment_seasons in a single batched query and grouping the results
// in Go, matching the store's read-many behavior (and its return-empty,
// never-nil-slice convention).
func (s *Store) assembleCommentRows(ctx context.Context, rows []database.Comment) ([]models.Comment, error) {
	if len(rows) == 0 {
		return []models.Comment{}, nil
	}

	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	seasonRows, err := s.qq(ctx).GetCommentSeasonsByCommentIds(ctx, ids)
	if err != nil {
		return []models.Comment{}, err
	}

	seasonsByComment := make(map[string][]database.CommentSeason, len(rows))
	for _, sr := range seasonRows {
		seasonsByComment[sr.CommentID] = append(seasonsByComment[sr.CommentID], sr)
	}

	comments := make([]models.Comment, len(rows))
	for i, r := range rows {
		comments[i] = commentRowToModel(r, assembleSeasonsComments(seasonsByComment[r.ID]))
	}
	return comments, nil
}

// AddComment inserts a new comment row plus its season rows (if any) in a
// single transaction: the id and timestamps
// are generated here, not taken from the caller-supplied comment.
func (s *Store) AddComment(ctx context.Context, comment models.Comment) (models.Comment, error) {
	var result models.Comment
	err := s.inTx(ctx, func(q *database.Queries) error {
		id := uuid.NewString()
		now := time.Now()

		row, err := q.InsertComment(ctx, database.InsertCommentParams{
			ID:        id,
			TitleID:   comment.TitleId,
			UserID:    comment.UserId,
			GroupID:   comment.GroupId,
			Comment:   ptrToText(comment.Comment),
			CreatedAt: timeToTimestamptz(now),
			UpdatedAt: timeToTimestamptz(now),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return store.ErrDuplicatedRecord
			}
			return err
		}

		if err := insertCommentSeasons(ctx, q, id, comment.SeasonsComments); err != nil {
			return err
		}

		result = commentRowToModel(row, comment.SeasonsComments)
		return nil
	})
	if err != nil {
		return models.Comment{}, err
	}
	return result, nil
}

// GetCommentsByTitleId fetches every comment left on titleId within groupId,
// seasons assembled. The comment row carries its group, so this filters on
// group_id directly rather than on the group's member list.
func (s *Store) GetCommentsByTitleId(ctx context.Context, titleId, groupId string) ([]models.Comment, error) {
	rows, err := s.qq(ctx).GetCommentRowsByTitleId(ctx, database.GetCommentRowsByTitleIdParams{
		TitleID: titleId,
		GroupID: groupId,
	})
	if err != nil {
		return []models.Comment{}, err
	}
	return s.assembleCommentRows(ctx, rows)
}

// GetUserCommentByTitleId fetches the (at most one) comment userId left on
// titleId within groupId, seasons assembled. groupId completes the key: the
// same user may hold a separate comment on the same title in another group.
func (s *Store) GetUserCommentByTitleId(ctx context.Context, titleId, userId, groupId string) (models.Comment, error) {
	row, err := s.qq(ctx).GetUserCommentRowByTitle(ctx, database.GetUserCommentRowByTitleParams{
		TitleID: titleId,
		UserID:  userId,
		GroupID: groupId,
	})
	if err != nil {
		return models.Comment{}, notFound(err)
	}

	seasonRows, err := s.qq(ctx).GetCommentSeasons(ctx, row.ID)
	if err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, assembleSeasonsComments(seasonRows)), nil
}

// GetCommentById fetches a single comment owned by userId, seasons assembled.
func (s *Store) GetCommentById(ctx context.Context, commentId string, userId string) (models.Comment, error) {
	row, err := s.qq(ctx).GetCommentRowById(ctx, database.GetCommentRowByIdParams{ID: commentId, UserID: userId})
	if err != nil {
		return models.Comment{}, notFound(err)
	}

	seasonRows, err := s.qq(ctx).GetCommentSeasons(ctx, row.ID)
	if err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, assembleSeasonsComments(seasonRows)), nil
}

// UpdateComment replaces the comment row's text plus its whole season set
// (delete-then-reinsert)'s document
// replace semantics for the seasonsComments field.
func (s *Store) UpdateComment(ctx context.Context, comment models.Comment, userId string) (models.Comment, error) {
	var result models.Comment
	err := s.inTx(ctx, func(q *database.Queries) error {
		row, err := q.UpdateCommentRow(ctx, database.UpdateCommentRowParams{
			ID:        comment.Id,
			UserID:    userId,
			Comment:   ptrToText(comment.Comment),
			UpdatedAt: timeToTimestamptz(time.Now()),
		})
		if err != nil {
			return notFound(err)
		}

		if err := q.DeleteCommentSeasons(ctx, row.ID); err != nil {
			return err
		}

		if err := insertCommentSeasons(ctx, q, row.ID, comment.SeasonsComments); err != nil {
			return err
		}

		result = commentRowToModel(row, comment.SeasonsComments)
		return nil
	})
	if err != nil {
		return models.Comment{}, err
	}
	return result, nil
}

// DeleteComment deletes the comment row owned by userId inside groupId
// (comment_seasons rows cascade): it just reports how many rows were affected,
// with no not-found error on a 0 count. groupId completes the key — an id that
// belongs to one of the user's comments in a different group matches nothing
// here, so a group-scoped route can never delete another group's comment.
func (s *Store) DeleteComment(ctx context.Context, commentId, userId, groupId string) (int64, error) {
	n, err := s.qq(ctx).DeleteCommentRow(ctx, database.DeleteCommentRowParams{ID: commentId, UserID: userId, GroupID: groupId})
	if err != nil {
		return 0, err
	}
	return n, nil
}
