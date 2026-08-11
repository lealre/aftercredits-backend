package postgres

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/store"
)

// newTestMovieTitle builds a minimal, valid models.Title (movie) for
// insertion, with the given id/primaryTitle/rating so ordering tests can
// control primary_title and rating_aggregate independently.
func newTestMovieTitle(t *testing.T, id, primaryTitle string, rating float64) models.Title {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return models.Title{
		ID:             id,
		Type:           "movie",
		PrimaryTitle:   primaryTitle,
		PrimaryImage:   models.Image{URL: "https://example.com/" + id + ".jpg", Width: 100, Height: 150},
		StartYear:      2000,
		RuntimeSeconds: 7200,
		Genres:         []string{"Drama"},
		Rating:         models.Rating{AggregateRating: rating, VoteCount: 1000},
		Plot:           "plot for " + id,
		AddedAt:        &now,
		UpdatedAt:      &now,
	}
}

// newTestSeriesTitle builds a fully-populated TV-series models.Title
// (seasons, episodes with both populated and nil optional fields, cast,
// metacritic, interests, ...) used to prove the JSONB round trip is
// byte-identical to the input.
func newTestSeriesTitle(t *testing.T) models.Title {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	runtime := 2400
	plot := "episode plot"

	return models.Title{
		ID:             "tt-series-" + uuid.NewString(),
		Type:           "tvSeries",
		PrimaryTitle:   "Test Series",
		PrimaryImage:   models.Image{URL: "https://example.com/series.jpg", Width: 300, Height: 450},
		StartYear:      2015,
		RuntimeSeconds: 2700,
		Genres:         []string{"Drama", "Crime"},
		Rating:         models.Rating{AggregateRating: 9.1, VoteCount: 54321},
		Metacritic:     &models.Metacritic{Score: 85, ReviewCount: 40},
		Plot:           "A gripping series about testing.",
		Directors: []models.Person{
			{
				ID:                 "nm1",
				DisplayName:        "Director One",
				AlternativeNames:   []string{"D One"},
				PrimaryImage:       &models.Image{URL: "https://example.com/d1.jpg", Width: 50, Height: 50},
				PrimaryProfessions: []string{"director"},
			},
		},
		Writers: []models.Person{
			{ID: "nm2", DisplayName: "Writer One", PrimaryProfessions: []string{"writer"}},
		},
		Stars: []models.Person{
			{ID: "nm3", DisplayName: "Star One", PrimaryProfessions: []string{"actor"}},
			{ID: "nm4", DisplayName: "Star Two", PrimaryProfessions: []string{"actress"}},
		},
		OriginCountries: []models.CodeName{{Code: "US", Name: "United States"}},
		SpokenLanguages: []models.CodeName{{Code: "en", Name: "English"}},
		Interests: []models.Interest{
			{ID: "int1", Name: "Crime Drama", IsSubgenre: true},
		},
		Seasons: []models.Seasons{
			{Season: "1", EpisodeCount: 10},
			{Season: "2", EpisodeCount: 8},
		},
		Episodes: []models.Episode{
			{
				ID:             "ep1",
				Title:          "Pilot",
				PrimaryImage:   models.Image{URL: "https://example.com/ep1.jpg", Width: 200, Height: 300},
				Season:         "1",
				EpisodeNumber:  1,
				RuntimeSeconds: &runtime,
				Plot:           &plot,
				Rating:         &models.Rating{AggregateRating: 8.5, VoteCount: 900},
				ReleaseDate:    &models.ReleaseDate{Year: 2015, Month: 3, Day: 1},
			},
			{
				// Deliberately leaves every optional field nil, to prove nil
				// conventions round-trip too, not just populated ones.
				ID:            "ep2",
				Title:         "Second",
				PrimaryImage:  models.Image{URL: "https://example.com/ep2.jpg", Width: 200, Height: 300},
				Season:        "1",
				EpisodeNumber: 2,
			},
		},
		AddedAt:   &now,
		UpdatedAt: &now,
	}
}

func TestStore_AddTitle_GetTitleById_RoundTrip(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	title := newTestSeriesTitle(t)
	require.NoError(t, s.AddTitle(ctx, title))

	got, err := s.GetTitleById(ctx, title.ID)
	require.NoError(t, err)
	require.Equal(t, title, got, "GetTitleById must return the exact models.Title that was added (metadata JSONB round trip)")
}

// TestStore_GetTitleById_NonNilEmptySlices proves that reading back a title
// whose optional slice fields were never set (a minimal movie, built via
// newTestMovieTitle, which leaves Directors/Writers/Stars/OriginCountries/
// SpokenLanguages/Interests/Seasons/Episodes as their nil zero value) still
// comes back with those 8 fields as non-nil, empty slices. This is a read-shape
// assertion against the store contract, not a round-trip-with-the-input check
// (the input here IS nil for these fields; the output must not be).
func TestStore_GetTitleById_NonNilEmptySlices(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	title := newTestMovieTitle(t, "tt-minimal", "Minimal Movie", 5.0)
	require.Nil(t, title.Directors, "sanity: fixture leaves Directors nil")
	require.Nil(t, title.Seasons, "sanity: fixture leaves Seasons nil")
	require.NoError(t, s.AddTitle(ctx, title))

	got, err := s.GetTitleById(ctx, title.ID)
	require.NoError(t, err)

	require.NotNil(t, got.Directors)
	require.Len(t, got.Directors, 0)
	require.NotNil(t, got.Writers)
	require.Len(t, got.Writers, 0)
	require.NotNil(t, got.Stars)
	require.Len(t, got.Stars, 0)
	require.NotNil(t, got.OriginCountries)
	require.Len(t, got.OriginCountries, 0)
	require.NotNil(t, got.SpokenLanguages)
	require.Len(t, got.SpokenLanguages, 0)
	require.NotNil(t, got.Interests)
	require.Len(t, got.Interests, 0)
	require.NotNil(t, got.Seasons)
	require.Len(t, got.Seasons, 0)
	require.NotNil(t, got.Episodes)
	require.Len(t, got.Episodes, 0)
}

func TestStore_AddTitle_Duplicate(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	title := newTestMovieTitle(t, "tt-dup", "Dup Movie", 7.0)
	require.NoError(t, s.AddTitle(ctx, title))

	err := s.AddTitle(ctx, title)
	require.ErrorIs(t, err, store.ErrDuplicatedRecord)
}

func TestStore_GetTitleById_NotFound(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetTitleById(ctx, "missing-title")
	require.ErrorIs(t, err, store.ErrRecordNotFound)
}

func TestStore_DeleteTitle(t *testing.T) {
	t.Run("deletes existing title", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		title := newTestMovieTitle(t, "tt-del", "Delete Me", 6.0)
		require.NoError(t, s.AddTitle(ctx, title))

		deleted, err := s.DeleteTitle(ctx, title.ID)
		require.NoError(t, err)
		require.True(t, deleted)

		_, err = s.GetTitleById(ctx, title.ID)
		require.ErrorIs(t, err, store.ErrRecordNotFound)
	})

	t.Run("missing title returns false", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		deleted, err := s.DeleteTitle(ctx, "does-not-exist")
		require.NoError(t, err)
		require.False(t, deleted)
	})
}

func TestStore_TitleExists(t *testing.T) {
	resetDB(t)
	s := newTestStore(t)
	ctx := context.Background()

	title := newTestMovieTitle(t, "tt-exists", "Exists", 5.0)

	exists, err := s.TitleExists(ctx, title.ID)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, s.AddTitle(ctx, title))

	exists, err = s.TitleExists(ctx, title.ID)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestStore_GetTitlesPage(t *testing.T) {
	t.Run("sorted by primary_title ascending with pagination", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		for _, ti := range []models.Title{
			newTestMovieTitle(t, "tt-a", "Alpha", 5.0),
			newTestMovieTitle(t, "tt-b", "Bravo", 6.0),
			newTestMovieTitle(t, "tt-c", "Charlie", 7.0),
		} {
			require.NoError(t, s.AddTitle(ctx, ti))
		}

		page1, total, err := s.GetTitlesPage(ctx, "primaryTitle", nil, 2, 1)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		require.Len(t, page1, 2)
		require.Equal(t, []string{"Alpha", "Bravo"}, []string{page1[0].PrimaryTitle, page1[1].PrimaryTitle})

		page2, total, err := s.GetTitlesPage(ctx, "primaryTitle", nil, 2, 2)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		require.Len(t, page2, 1)
		require.Equal(t, "Charlie", page2[0].PrimaryTitle)
	})

	t.Run("sorted by primary_title descending", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, "tt-a", "Alpha", 5.0)))
		require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, "tt-b", "Bravo", 6.0)))

		descending := false
		got, total, err := s.GetTitlesPage(ctx, "primaryTitle", &descending, 10, 1)
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Equal(t, []string{"Bravo", "Alpha"}, []string{got[0].PrimaryTitle, got[1].PrimaryTitle})
	})

	t.Run("sorted by imdbRating", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, "tt-low", "Low Rated", 3.0)))
		require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, "tt-high", "High Rated", 9.0)))

		got, _, err := s.GetTitlesPage(ctx, "imdbRating", nil, 10, 1)
		require.NoError(t, err)
		require.Equal(t, []string{"tt-low", "tt-high"}, []string{got[0].ID, got[1].ID})
	})

	// Regression: the ORDER BY that feeds LIMIT/OFFSET had no tie-break, so rows
	// that compared equal on the sort column had no defined relative order and
	// Postgres could place one of them differently for each page (the bound it
	// sorts under changes with the OFFSET). Walking the pages then returned one
	// row twice and another never — observed on real data, 122 titles ordered by
	// rating descending yielding 119 rows and 118 distinct titles.
	t.Run("paging a heavily tied sort returns every row exactly once", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		// 200 rows over a handful of distinct values per column: every sort key
		// has large blocks of rows that compare equal.
		const total = 200
		const pageSize = 7
		want := make([]string, 0, total)
		for i := range total {
			title := newTestMovieTitle(t,
				fmt.Sprintf("tt%05d", i),
				fmt.Sprintf("Tied %d", i%3),
				float64(5+i%4),
			)
			title.Rating.VoteCount = 100 * (i % 5)
			title.StartYear = 2000 + i%2
			// updated_at is the one nullable sort column; NULL rows tie too.
			if i%2 == 0 {
				title.UpdatedAt = nil
			}
			require.NoError(t, s.AddTitle(ctx, title))
			want = append(want, title.ID)
		}

		for _, orderBy := range []string{"", "primaryTitle", "imdbRating", "startYear", "type", "voteCount", "addedAt", "updatedAt"} {
			for _, ascending := range []bool{true, false} {
				direction := ascending
				var got []string
				for page := 1; ; page++ {
					titles, count, err := s.GetTitlesPage(ctx, orderBy, &direction, pageSize, page)
					require.NoError(t, err, "paging by %q failed", orderBy)
					require.EqualValues(t, total, count, "the total must not move while paging by %q", orderBy)
					if len(titles) == 0 {
						break
					}
					for _, title := range titles {
						got = append(got, title.ID)
					}
				}

				require.ElementsMatch(t, want, got,
					"paging by %q (ascending=%v) must return every row exactly once", orderBy, direction)
			}
		}
	})

	t.Run("an extreme page number returns an empty page with the correct total instead of erroring", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		names := []string{"Alpha", "Bravo", "Charlie"}
		for i, name := range names {
			require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, fmt.Sprintf("tt-extreme-%d", i), name, 5.0)))
		}

		cases := []struct {
			name string
			page int
		}{
			// page-1 * size wraps to a small POSITIVE int64 (92) if the guard
			// multiplies first — which would silently return some unrelated
			// page's rows instead of emptying out.
			{"page-1 * size wraps to a small positive int64", 92_233_720_368_547_760},
			// page-1 is near MaxInt64; multiplying by size=100 in int64 BEFORE
			// the overflow guard would itself wrap to a small negative int64,
			// which Postgres rejects with "OFFSET must not be negative", a
			// 500. The guard must reject this by comparing before any
			// multiply, not after.
			{"page-1 near MaxInt64: int64 multiply would wrap negative", math.MaxInt64},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, total, err := s.GetTitlesPage(ctx, "", nil, 100, tc.page)
				require.NoError(t, err)
				require.Equal(t, []models.Title{}, got)
				require.EqualValues(t, len(names), total)
			})
		}
	})

	// The doc comment on GetTitlesPage makes clamping the service's job, so
	// size arrives unvalidated — and this method is reachable directly (this
	// file, cmd/routines). A non-positive size must page to nothing, the way
	// it did when size was only ever bound as a LIMIT parameter. size == 0 in
	// particular must never reach the offset guard's division by size: this
	// process has no panic-recovery middleware, so an integer divide-by-zero
	// there takes the whole server down rather than failing one request.
	t.Run("a non-positive size returns an empty page with the correct total", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		names := []string{"Alpha", "Bravo", "Charlie"}
		for i, name := range names {
			require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, fmt.Sprintf("tt-size-%d", i), name, 5.0)))
		}

		for _, size := range []int{0, -1, math.MinInt64} {
			t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
				for _, page := range []int{1, 2, math.MaxInt64} {
					// Asserted through NotPanics rather than called directly:
					// without the guard this is an integer divide-by-zero, and
					// an unrecovered panic aborts the whole test binary instead
					// of reporting which case regressed.
					var (
						got   []models.Title
						total int64
						err   error
					)
					require.NotPanics(t, func() {
						got, total, err = s.GetTitlesPage(ctx, "", nil, size, page)
					}, "size=%d page=%d must not panic", size, page)
					require.NoError(t, err, "size=%d page=%d must not error", size, page)
					require.Equal(t, []models.Title{}, got, "size=%d page=%d must page to nothing", size, page)
					require.EqualValues(t, len(names), total, "size=%d page=%d must still report the real total", size, page)
				}
			})
		}
	})

	// MAX_PAGE_SIZE is env-configurable with no upper bound, so the clamped
	// size the service passes down can legitimately exceed int32. Narrowing it
	// to the generated LIMIT parameter used to wrap it negative, and Postgres
	// rejects a negative LIMIT outright — every request would 500. page_size
	// is a bigint for exactly this reason.
	t.Run("a size larger than int32 is not narrowed", func(t *testing.T) {
		resetDB(t)
		s := newTestStore(t)
		ctx := context.Background()

		names := []string{"Alpha", "Bravo", "Charlie"}
		for i, name := range names {
			require.NoError(t, s.AddTitle(ctx, newTestMovieTitle(t, fmt.Sprintf("tt-wide-%d", i), name, 5.0)))
		}

		got, total, err := s.GetTitlesPage(ctx, "", nil, math.MaxInt32+1, 1)
		require.NoError(t, err, "a size past int32 must not wrap into a negative LIMIT")
		require.Len(t, got, len(names), "a size that large must simply return every row")
		require.EqualValues(t, len(names), total)
	})
}

func TestListTitleIds(t *testing.T) {
	resetDB(t)
	st := newTestStore(t)
	ctx := context.Background()

	ids, err := st.ListTitleIds(ctx)
	require.NoError(t, err)
	require.Empty(t, ids)

	require.NoError(t, st.AddTitle(ctx, models.Title{ID: "tt0000002", PrimaryTitle: "B"}))
	require.NoError(t, st.AddTitle(ctx, models.Title{ID: "tt0000001", PrimaryTitle: "A"}))

	ids, err = st.ListTitleIds(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"tt0000001", "tt0000002"}, ids)
}

func TestUpdateTitle(t *testing.T) {
	resetDB(t)
	st := newTestStore(t)
	ctx := context.Background()

	orig := models.Title{ID: "tt0000001", Type: "movie", PrimaryTitle: "Before",
		Rating: models.Rating{AggregateRating: 7.0, VoteCount: 10}}
	require.NoError(t, st.AddTitle(ctx, orig))

	now := time.Now().UTC().Truncate(time.Millisecond)
	updated := orig
	updated.PrimaryTitle = "After"
	updated.Rating = models.Rating{AggregateRating: 8.5, VoteCount: 999}
	updated.UpdatedAt = &now

	require.NoError(t, st.UpdateTitle(ctx, updated))

	got, err := st.GetTitleById(ctx, "tt0000001")
	require.NoError(t, err)
	require.Equal(t, "After", got.PrimaryTitle)
	require.Equal(t, 8.5, got.Rating.AggregateRating)
	require.NotNil(t, got.UpdatedAt)
	require.True(t, got.UpdatedAt.Equal(now))

	// The denormalized query column must be updated too (sort path).
	var col string
	require.NoError(t, newTestPool(t).QueryRow(ctx,
		"SELECT primary_title FROM titles WHERE id = $1", "tt0000001").Scan(&col))
	require.Equal(t, "After", col)

	require.ErrorIs(t, st.UpdateTitle(ctx, models.Title{ID: "tt-none"}), store.ErrRecordNotFound)
}
