package pgmigration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lealre/movies-backend/internal/database"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/postgres"
)

// ErrTargetNotEmpty is returned by Load when the target database already
// holds data and reset was not requested.
var ErrTargetNotEmpty = errors.New("target database is not empty (re-run with --reset to truncate and reload)")

// allTables lists every table the migration owns. Fixed at compile time —
// the names are interpolated into COUNT/TRUNCATE statements.
var allTables = []string{
	"users", "titles", "ratings", "rating_seasons", "comments",
	"comment_seasons", "groups", "group_members", "group_titles",
	"group_title_seasons",
}

// LoadStats counts the rows written per table.
type LoadStats struct {
	Users, Titles, Ratings, RatingSeasons, Comments, CommentSeasons int
	Groups, GroupMembers, GroupTitles, GroupTitleSeasons            int
}

// Load writes the whole dump into Postgres inside a single transaction, in
// FK-safe order (users -> titles -> groups -> ratings -> comments),
// preserving every original id and timestamp. If the target holds any rows
// it fails with ErrTargetNotEmpty unless reset is true, in which case all
// tables are truncated inside the same transaction. On any error nothing is
// committed.
//
// The store's AddRating/AddComment/CreateGroup mint fresh uuids and stamp
// now(), so everything goes through the sqlc queries directly; param
// building for titles reuses postgres.TitleToRow so the JSONB metadata is
// byte-identical to what the store writes.
func Load(ctx context.Context, pool *pgxpool.Pool, dump *Dump, reset bool) (LoadStats, error) {
	var stats LoadStats

	tx, err := pool.Begin(ctx)
	if err != nil {
		return stats, err
	}
	defer tx.Rollback(ctx)

	total, err := countAllRows(ctx, tx)
	if err != nil {
		return stats, err
	}
	if total > 0 {
		if !reset {
			return stats, ErrTargetNotEmpty
		}
		if _, err := tx.Exec(ctx, "TRUNCATE "+strings.Join(allTables, ", ")); err != nil {
			return stats, fmt.Errorf("truncate: %w", err)
		}
	}

	q := database.New(tx)

	for _, u := range dump.Users {
		if err := q.CreateUser(ctx, database.CreateUserParams{
			ID:           u.Id,
			Name:         u.Name,
			Email:        u.Email,
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			AvatarUrl:    postgres.PtrToText(u.AvatarURL),
			Role:         string(u.Role),
			IsActive:     u.IsActive,
			LastLoginAt:  postgres.PtrToTimestamptz(u.LastLoginAt),
			CreatedAt:    postgres.TimeToTimestamptz(u.CreatedAt),
			UpdatedAt:    postgres.TimeToTimestamptz(u.UpdatedAt),
		}); err != nil {
			return stats, fmt.Errorf("insert user %s: %w", u.Id, err)
		}
		stats.Users++
	}

	for _, t := range dump.Titles {
		params, err := postgres.TitleToRow(mongodb.TitleDbToModel(t))
		if err != nil {
			return stats, fmt.Errorf("map title %s: %w", t.ID, err)
		}
		if err := q.InsertTitle(ctx, params); err != nil {
			return stats, fmt.Errorf("insert title %s: %w", t.ID, err)
		}
		stats.Titles++
	}

	for _, g := range dump.Groups {
		if err := q.InsertGroupFull(ctx, database.InsertGroupFullParams{
			ID:          g.Id,
			Name:        g.Name,
			Description: g.Description,
			OwnerID:     g.OwnerId,
			Deleted:     g.Deleted,
			DeletedAt:   postgres.PtrToTimestamptz(g.DeletedAt),
			CreatedAt:   postgres.TimeToTimestamptz(g.CreatedAt),
			UpdatedAt:   postgres.TimeToTimestamptz(g.UpdatedAt),
		}); err != nil {
			return stats, fmt.Errorf("insert group %s: %w", g.Id, err)
		}
		stats.Groups++

		for _, userId := range g.Users {
			if err := q.AddGroupMember(ctx, database.AddGroupMemberParams{
				GroupID: g.Id,
				UserID:  userId,
			}); err != nil {
				return stats, fmt.Errorf("insert member %s of group %s: %w", userId, g.Id, err)
			}
			stats.GroupMembers++
		}

		for titleId, item := range g.Titles {
			if _, err := q.UpsertGroupTitle(ctx, database.UpsertGroupTitleParams{
				GroupID:   g.Id,
				TitleID:   string(titleId),
				TitleType: item.TitleType,
				Watched:   item.Watched,
				WatchedAt: postgres.PtrToTimestamptz(item.WatchedAt),
				AddedAt:   postgres.TimeToTimestamptz(item.AddedAt),
				UpdatedAt: postgres.TimeToTimestamptz(item.UpdatedAt),
			}); err != nil {
				return stats, fmt.Errorf("insert title %s of group %s: %w", titleId, g.Id, err)
			}
			stats.GroupTitles++

			if item.SeasonsWatched == nil {
				continue
			}
			for season, sw := range *item.SeasonsWatched {
				if _, err := q.UpsertGroupTitleSeason(ctx, database.UpsertGroupTitleSeasonParams{
					GroupID:   g.Id,
					TitleID:   string(titleId),
					Season:    season,
					Watched:   sw.Watched,
					WatchedAt: postgres.PtrToTimestamptz(sw.WatchedAt),
					AddedAt:   postgres.TimeToTimestamptz(sw.AddedAt),
					UpdatedAt: postgres.TimeToTimestamptz(sw.UpdatedAt),
				}); err != nil {
					return stats, fmt.Errorf("insert season %s of group %s title %s: %w", season, g.Id, titleId, err)
				}
				stats.GroupTitleSeasons++
			}
		}
	}

	for _, r := range dump.Ratings {
		if _, err := q.InsertRating(ctx, database.InsertRatingParams{
			ID:        r.Id,
			TitleID:   r.TitleId,
			UserID:    r.UserId,
			Note:      r.Note,
			CreatedAt: postgres.TimeToTimestamptz(r.CreatedAt),
			UpdatedAt: postgres.TimeToTimestamptz(r.UpdatedAt),
		}); err != nil {
			return stats, fmt.Errorf("insert rating %s: %w", r.Id, err)
		}
		stats.Ratings++

		if r.SeasonsRatings == nil {
			continue
		}
		for season, sr := range *r.SeasonsRatings {
			if err := q.InsertRatingSeason(ctx, database.InsertRatingSeasonParams{
				RatingID:  r.Id,
				Season:    season,
				Rating:    sr.Rating,
				AddedAt:   postgres.TimeToTimestamptz(sr.AddedAt),
				UpdatedAt: postgres.TimeToTimestamptz(sr.UpdatedAt),
			}); err != nil {
				return stats, fmt.Errorf("insert season %s of rating %s: %w", season, r.Id, err)
			}
			stats.RatingSeasons++
		}
	}

	for _, c := range dump.Comments {
		if _, err := q.InsertComment(ctx, database.InsertCommentParams{
			ID:        c.Id,
			TitleID:   c.TitleId,
			UserID:    c.UserId,
			Comment:   postgres.PtrToText(c.Comment),
			CreatedAt: postgres.TimeToTimestamptz(c.CreatedAt),
			UpdatedAt: postgres.TimeToTimestamptz(c.UpdatedAt),
		}); err != nil {
			return stats, fmt.Errorf("insert comment %s: %w", c.Id, err)
		}
		stats.Comments++

		if c.SeasonsComments == nil {
			continue
		}
		for season, sc := range *c.SeasonsComments {
			if err := q.InsertCommentSeason(ctx, database.InsertCommentSeasonParams{
				CommentID: c.Id,
				Season:    season,
				Comment:   sc.Comment,
				AddedAt:   postgres.TimeToTimestamptz(sc.AddedAt),
				UpdatedAt: postgres.TimeToTimestamptz(sc.UpdatedAt),
			}); err != nil {
				return stats, fmt.Errorf("insert season %s of comment %s: %w", season, c.Id, err)
			}
			stats.CommentSeasons++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return stats, err
	}
	return stats, nil
}

// countAllRows sums the row counts of every migration-owned table, inside
// the load transaction so the emptiness check and the writes are atomic.
func countAllRows(ctx context.Context, tx pgx.Tx) (int64, error) {
	var total int64
	for _, table := range allTables {
		var n int64
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			return 0, fmt.Errorf("count %s: %w", table, err)
		}
		total += n
	}
	return total, nil
}
