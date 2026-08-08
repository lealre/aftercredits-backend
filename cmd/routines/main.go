package main

import (
	"context"
	"errors"
	"log"
	"reflect"
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
