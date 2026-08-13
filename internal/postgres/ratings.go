package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// insertRatingSeasons inserts every entry of seasons (if any) as a
// rating_seasons row belonging to ratingId, using qtx (a possibly
// tx-bound *database.Queries).
func insertRatingSeasons(ctx context.Context, qtx *database.Queries, ratingId string, seasons *models.SeasonsRatings) error {
	if seasons == nil {
		return nil
	}
	for season, item := range *seasons {
		if err := qtx.InsertRatingSeason(ctx, database.InsertRatingSeasonParams{
			RatingID:  ratingId,
			Season:    season,
			Rating:    item.Rating,
			AddedAt:   timeToTimestamptz(item.AddedAt),
			UpdatedAt: timeToTimestamptz(item.UpdatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

// assembleRatingRows builds a models.UserRating for each row, fetching all
// of their rating_seasons in a single batched query and grouping the
// results in Go, matching the store's read-many behavior (and its
// return-empty, never-nil-slice convention).
func (s *Store) assembleRatingRows(ctx context.Context, rows []database.Rating) ([]models.UserRating, error) {
	if len(rows) == 0 {
		return []models.UserRating{}, nil
	}

	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	seasonRows, err := s.q.GetRatingSeasonsByRatingIds(ctx, ids)
	if err != nil {
		return []models.UserRating{}, err
	}

	seasonsByRating := make(map[string][]database.RatingSeason, len(rows))
	for _, sr := range seasonRows {
		seasonsByRating[sr.RatingID] = append(seasonsByRating[sr.RatingID], sr)
	}

	ratings := make([]models.UserRating, len(rows))
	for i, r := range rows {
		ratings[i] = ratingRowToModel(r, assembleSeasonsRatings(seasonsByRating[r.ID]))
	}
	return ratings, nil
}

// AddRating inserts a new rating row plus its season rows (if any) in a
// single transaction: the id and timestamps
// are generated here, not taken from the caller-supplied rating.
func (s *Store) AddRating(ctx context.Context, rating models.UserRating) (models.UserRating, error) {
	var result models.UserRating
	err := s.inTx(ctx, func(q *database.Queries) error {
		id := uuid.NewString()
		now := time.Now()

		row, err := q.InsertRating(ctx, database.InsertRatingParams{
			ID:        id,
			TitleID:   rating.TitleId,
			UserID:    rating.UserId,
			GroupID:   rating.GroupId,
			Note:      rating.Note,
			CreatedAt: timeToTimestamptz(now),
			UpdatedAt: timeToTimestamptz(now),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return store.ErrDuplicatedRecord
			}
			return err
		}

		if err := insertRatingSeasons(ctx, q, id, rating.SeasonsRatings); err != nil {
			return err
		}

		result = ratingRowToModel(row, rating.SeasonsRatings)
		return nil
	})
	if err != nil {
		return models.UserRating{}, err
	}
	return result, nil
}

// GetRatingsByTitleId fetches every rating left on titleId within groupId,
// seasons assembled. Ratings are group-scoped, so a title's ratings are only
// ever read through the group they belong to.
func (s *Store) GetRatingsByTitleId(ctx context.Context, titleId, groupId string) ([]models.UserRating, error) {
	rows, err := s.q.GetRatingRowsByTitleId(ctx, database.GetRatingRowsByTitleIdParams{
		TitleID: titleId,
		GroupID: groupId,
	})
	if err != nil {
		return []models.UserRating{}, err
	}
	return s.assembleRatingRows(ctx, rows)
}

// GetRatingById fetches a single rating owned by userId, seasons assembled.
func (s *Store) GetRatingById(ctx context.Context, ratingId, userId string) (models.UserRating, error) {
	row, err := s.q.GetRatingRowById(ctx, database.GetRatingRowByIdParams{ID: ratingId, UserID: userId})
	if err != nil {
		return models.UserRating{}, notFound(err)
	}

	seasonRows, err := s.q.GetRatingSeasons(ctx, row.ID)
	if err != nil {
		return models.UserRating{}, err
	}

	return ratingRowToModel(row, assembleSeasonsRatings(seasonRows)), nil
}

// GetRatingByUserIdAndTitleId fetches the (at most one) rating userId left on
// titleId within groupId, seasons assembled. groupId completes the key: the
// same user may hold a separate rating of the same title in another group.
func (s *Store) GetRatingByUserIdAndTitleId(ctx context.Context, userId, titleId, groupId string) (models.UserRating, error) {
	row, err := s.q.GetRatingRowByUserTitle(ctx, database.GetRatingRowByUserTitleParams{
		UserID:  userId,
		TitleID: titleId,
		GroupID: groupId,
	})
	if err != nil {
		return models.UserRating{}, notFound(err)
	}

	seasonRows, err := s.q.GetRatingSeasons(ctx, row.ID)
	if err != nil {
		return models.UserRating{}, err
	}

	return ratingRowToModel(row, assembleSeasonsRatings(seasonRows)), nil
}

// UpdateRating replaces the rating row's note plus its whole season set
// (delete-then-reinsert)'s document
// replace semantics for the seasonsRatings field.
func (s *Store) UpdateRating(ctx context.Context, rating models.UserRating, userId string) (models.UserRating, error) {
	var result models.UserRating
	err := s.inTx(ctx, func(q *database.Queries) error {
		row, err := q.UpdateRatingRow(ctx, database.UpdateRatingRowParams{
			ID:        rating.Id,
			UserID:    userId,
			Note:      rating.Note,
			UpdatedAt: timeToTimestamptz(time.Now()),
		})
		if err != nil {
			return notFound(err)
		}

		if err := q.DeleteRatingSeasons(ctx, row.ID); err != nil {
			return err
		}

		if err := insertRatingSeasons(ctx, q, row.ID, rating.SeasonsRatings); err != nil {
			return err
		}

		result = ratingRowToModel(row, rating.SeasonsRatings)
		return nil
	})
	if err != nil {
		return models.UserRating{}, err
	}
	return result, nil
}

// GetRatingsByTitleIds fetches every rating left on the given titleIds within
// groupId in one batch, seasons assembled via a single grouped query. The
// groupId filter is load-bearing: without it this read serves one group's
// title detail every other group's ratings.
func (s *Store) GetRatingsByTitleIds(ctx context.Context, titleIds []string, groupId string) ([]models.UserRating, error) {
	rows, err := s.q.GetRatingRowsByTitleIds(ctx, database.GetRatingRowsByTitleIdsParams{
		Column1: titleIds,
		GroupID: groupId,
	})
	if err != nil {
		return []models.UserRating{}, err
	}
	return s.assembleRatingRows(ctx, rows)
}

// DeleteRating deletes the rating row owned by userId (rating_seasons rows
// cascade): no rows affected is reported as
// store.ErrRecordNotFound rather than (0, nil).
func (s *Store) DeleteRating(ctx context.Context, ratingId, userId string) (int64, error) {
	n, err := s.q.DeleteRatingRow(ctx, database.DeleteRatingRowParams{ID: ratingId, UserID: userId})
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, store.ErrRecordNotFound
	}
	return n, nil
}
