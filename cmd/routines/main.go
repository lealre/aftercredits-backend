package main

import (
	"context"
	"errors"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/postgres"
	"github.com/lealre/movies-backend/internal/services/titles"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

func main() {
	_ = godotenv.Load()

	log.Println("")
	log.Println("==========================================")
	log.Println("🎬 Starting titles update...")
	log.Println("==========================================")

	provider, err := factory.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to build title provider: %v", err)
	}
	log.Printf("Using title provider: %s", provider.Name())

	ctx := context.Background()
	pool, err := postgres.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pool.Close()

	st := postgres.New(pool)

	log.Println("Fetching all title IDs from database...")
	titleIDs, err := st.ListTitleIds(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch title IDs: %v", err)
	}
	log.Printf("Found %d titles to sync", len(titleIDs))

	if err := syncTitles(ctx, provider, st, titleIDs); err != nil {
		log.Fatalf("Failed to sync titles: %v", err)
	}
	log.Println("Sync completed successfully")

	pruneActivityEvents(ctx, st)
}

// activityRetentionEnabled reports whether the prune below is allowed to delete
// anything at all. It is OFF unless explicitly switched on.
//
// The feed is an append-only log, and the owner's position is that an event is
// worth keeping until there is a reason not to — a watchlist for a handful of
// people produces a trickle of rows, so unbounded growth is a theoretical
// problem here long before it is a real one. Deleting history is also the one
// operation in this job that cannot be undone: a bad cutoff, a clock skew or a
// mistyped ACTIVITY_RETENTION_DAYS destroys events with no backup of their own.
// So the default is to keep everything, and turning pruning on is a deliberate
// act rather than something that happens because nobody looked at the default.
//
// Set ACTIVITY_RETENTION_ENABLED to one of true/1/yes/on to enable it.
func activityRetentionEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ACTIVITY_RETENTION_ENABLED"))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// activityRetentionDays is how long an activity event is kept ONCE pruning is
// enabled. Ignored entirely while activityRetentionEnabled reports false.
// Overridable via ACTIVITY_RETENTION_DAYS.
func activityRetentionDays() int {
	if v := strings.TrimSpace(os.Getenv("ACTIVITY_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 90
}

// pruneActivityEvents deletes activity events past the retention window. A
// failure here is logged, not fatal: it must never take down the title sync
// that is this job's primary purpose.
func pruneActivityEvents(ctx context.Context, st *postgres.Store) {
	if !activityRetentionEnabled() {
		// Said out loud on every run. A retention job that silently does nothing
		// is indistinguishable from one that is silently broken, and the day
		// someone turns this on they will want to find this line in the log.
		log.Println("Activity retention is disabled (ACTIVITY_RETENTION_ENABLED is not set); keeping all events")
		return
	}

	cutoff := time.Now().AddDate(0, 0, -activityRetentionDays())
	log.Printf("Pruning activity events older than %d days (before %s)...", activityRetentionDays(), cutoff.Format(time.RFC3339))
	deleted, err := st.DeleteActivityEventsOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("WARN: failed to prune activity events: %v", err)
		return
	}
	log.Printf("Pruned %d activity events", deleted)
}

func syncTitles(ctx context.Context, provider titleprovider.Provider, st *postgres.Store, titleIDs []string) error {
	jobs := make(chan string, len(titleIDs))
	wg := sync.WaitGroup{}
	workerCount := 5

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for titleID := range jobs {
				if err := processTitle(ctx, provider, st, titleID); err != nil {
					log.Printf("failed processing %s: %v", titleID, err)
				}
			}
		}()
	}

	for _, id := range titleIDs {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	return nil
}

func processTitle(ctx context.Context, provider titleprovider.Provider, st *postgres.Store, titleID string) error {
	dbTitle, err := st.GetTitleById(ctx, titleID)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			log.Printf("Title %s not found in database, skipping", titleID)
			return nil
		}
		return err
	}

	apiTitle, err := provider.GetTitle(ctx, titleID)
	if err != nil {
		return err
	}

	updated, changed := refreshTitle(dbTitle, apiTitle, time.Now())
	if err := st.UpdateTitle(ctx, updated); err != nil {
		return err
	}

	if changed {
		log.Printf("Updated title %s (fields changed)", titleID)
	} else {
		log.Printf("Updated title %s (updatedAt only)", titleID)
	}
	return nil
}

// refreshTitle applies the provider's current values for the synced fields
// (primaryImage, seasons, episodes, rating, metacritic) onto the stored
// title, always stamping UpdatedAt with now — the same semantics the previous
// routines had. The bool reports whether any content field changed.
func refreshTitle(dbTitle models.Title, apiTitle *titleprovider.Title, now time.Time) (models.Title, bool) {
	changed := false

	apiImage := models.Image{URL: apiTitle.PrimaryImage.URL, Width: apiTitle.PrimaryImage.Width, Height: apiTitle.PrimaryImage.Height}
	if dbTitle.PrimaryImage != apiImage {
		dbTitle.PrimaryImage = apiImage
		changed = true
	}

	apiSeasons := titles.MapImdbSeasonsToDbSeasons(apiTitle.Seasons)
	if !slicesEqual(dbTitle.Seasons, apiSeasons) {
		dbTitle.Seasons = apiSeasons
		changed = true
	}

	apiEpisodes := titles.MapImdbEpisodesToDbEpisodes(apiTitle.Episodes)
	if !slicesEqual(dbTitle.Episodes, apiEpisodes) {
		dbTitle.Episodes = apiEpisodes
		changed = true
	}

	apiRating := models.Rating{AggregateRating: apiTitle.Rating.AggregateRating, VoteCount: apiTitle.Rating.VoteCount}
	if dbTitle.Rating != apiRating {
		dbTitle.Rating = apiRating
		changed = true
	}

	var apiMetacritic *models.Metacritic
	if apiTitle.Metacritic != nil {
		apiMetacritic = &models.Metacritic{Score: apiTitle.Metacritic.Score, ReviewCount: apiTitle.Metacritic.ReviewCount}
	}
	if !metacriticEqual(dbTitle.Metacritic, apiMetacritic) {
		dbTitle.Metacritic = apiMetacritic
		changed = true
	}

	dbTitle.UpdatedAt = &now
	return dbTitle, changed
}

func metacriticEqual(a, b *models.Metacritic) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Score == b.Score && a.ReviewCount == b.ReviewCount
}

// slicesEqual is an honest content comparison: unlike the previous
// reflect.DeepEqual across mismatched types (storage vs domain types, always
// false), this compares like-typed models slices for real. It treats nil
// and empty as equivalent — MapImdbSeasonsToDbSeasons/MapImdbEpisodesToDbEpisodes
// return a non-nil empty slice via make() when the provider has none, which
// would otherwise spuriously differ from an unset (nil) stored slice.
func slicesEqual[T any](a, b []T) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
