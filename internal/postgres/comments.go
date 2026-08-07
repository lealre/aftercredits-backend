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

	seasonRows, err := s.q.GetCommentSeasonsByCommentIds(ctx, ids)
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Comment{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	id := uuid.NewString()
	now := time.Now()

	row, err := qtx.InsertComment(ctx, database.InsertCommentParams{
		ID:        id,
		TitleID:   comment.TitleId,
		UserID:    comment.UserId,
		Comment:   ptrToText(comment.Comment),
		CreatedAt: timeToTimestamptz(now),
		UpdatedAt: timeToTimestamptz(now),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return models.Comment{}, store.ErrDuplicatedRecord
		}
		return models.Comment{}, err
	}

	if err := insertCommentSeasons(ctx, qtx, id, comment.SeasonsComments); err != nil {
		return models.Comment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, comment.SeasonsComments), nil
}

// GetCommentsByTitleId fetches every comment for titleId left by a user in
// usersFromGroup, seasons assembled.
func (s *Store) GetCommentsByTitleId(ctx context.Context, titleId string, usersFromGroup []string) ([]models.Comment, error) {
	rows, err := s.q.GetCommentRowsByTitleId(ctx, database.GetCommentRowsByTitleIdParams{
		TitleID: titleId,
		Column2: usersFromGroup,
	})
	if err != nil {
		return []models.Comment{}, err
	}
	return s.assembleCommentRows(ctx, rows)
}

// GetUserCommentByTitleId fetches the (at most one) comment userId left on
// titleId, seasons assembled.
func (s *Store) GetUserCommentByTitleId(ctx context.Context, titleId string, userId string) (models.Comment, error) {
	row, err := s.q.GetUserCommentRowByTitle(ctx, database.GetUserCommentRowByTitleParams{TitleID: titleId, UserID: userId})
	if err != nil {
		return models.Comment{}, notFound(err)
	}

	seasonRows, err := s.q.GetCommentSeasons(ctx, row.ID)
	if err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, assembleSeasonsComments(seasonRows)), nil
}

// GetCommentById fetches a single comment owned by userId, seasons assembled.
func (s *Store) GetCommentById(ctx context.Context, commentId string, userId string) (models.Comment, error) {
	row, err := s.q.GetCommentRowById(ctx, database.GetCommentRowByIdParams{ID: commentId, UserID: userId})
	if err != nil {
		return models.Comment{}, notFound(err)
	}

	seasonRows, err := s.q.GetCommentSeasons(ctx, row.ID)
	if err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, assembleSeasonsComments(seasonRows)), nil
}

// UpdateComment replaces the comment row's text plus its whole season set
// (delete-then-reinsert)'s document
// replace semantics for the seasonsComments field.
func (s *Store) UpdateComment(ctx context.Context, comment models.Comment, userId string) (models.Comment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Comment{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	row, err := qtx.UpdateCommentRow(ctx, database.UpdateCommentRowParams{
		ID:        comment.Id,
		UserID:    userId,
		Comment:   ptrToText(comment.Comment),
		UpdatedAt: timeToTimestamptz(time.Now()),
	})
	if err != nil {
		return models.Comment{}, notFound(err)
	}

	if err := qtx.DeleteCommentSeasons(ctx, row.ID); err != nil {
		return models.Comment{}, err
	}

	if err := insertCommentSeasons(ctx, qtx, row.ID, comment.SeasonsComments); err != nil {
		return models.Comment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Comment{}, err
	}

	return commentRowToModel(row, comment.SeasonsComments), nil
}

// DeleteComment deletes the comment row owned by userId (comment_seasons
// rows cascade): it just reports how many
// rows were affected, with no not-found error on a 0 count.
func (s *Store) DeleteComment(ctx context.Context, commentId, userId string) (int64, error) {
	n, err := s.q.DeleteCommentRow(ctx, database.DeleteCommentRowParams{ID: commentId, UserID: userId})
	if err != nil {
		return 0, err
	}
	return n, nil
}
