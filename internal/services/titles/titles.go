package titles

import (
	"context"
	"errors"
	"time"

	"github.com/lealre/movies-backend/internal/config"
	"github.com/lealre/movies-backend/internal/generics"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/store"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

// GetPageOfTitles returns a page of titles from the native titles
// collection, sorted by orderByField/ascending. Used by the admin titles
// listing, which pages over every title (there is no filtering).
func GetPageOfTitles(
	db store.Store,
	ctx context.Context,
	size, page int,
	orderByField string,
	ascending *bool,
) (generics.Page[Title], error) {

	// App-level pagination normalization - stays in the service, it has
	// nothing to do with the storage backend. Shared with the group-titles
	// listing so the two cannot drift.
	size, page = config.NormalizePageParams(size, page)

	// Everything storage-specific (filter/pipeline construction, the two
	// sort strategies, and the orderBy field remapping) lives in the store.
	titlesModel, totalResults, err := db.GetTitlesPage(ctx, orderByField, ascending, size, page)
	if err != nil {
		return generics.Page[Title]{}, err
	}

	titles := make([]Title, len(titlesModel))
	for i, t := range titlesModel {
		titles[i] = MapDbTitleToApiTitle(t)
	}

	return generics.Page[Title]{
		TotalResults: int(totalResults),
		Size:         size,
		Page:         page,
		TotalPages:   int((totalResults + int64(size) - 1) / int64(size)),
		Content:      titles,
	}, nil
}

func AddNewTitle(db store.Store, provider titleprovider.Provider, ctx context.Context, titleId string) (Title, error) {
	logger := logx.FromContext(ctx)

	providerTitle, err := provider.GetTitle(ctx, titleId)
	if err != nil {
		// Translate the provider's not-found into the titles service vocabulary
		// so handlers map it via titles.ErrorMap (not the provider's error).
		if errors.Is(err, titleprovider.ErrTitleNotFound) {
			return Title{}, ErrTitleNotFound
		}
		return Title{}, err
	}
	if providerTitle.Type == "tvSeries" || providerTitle.Type == "tvMiniSeries" {
		logger.Printf("Title %s is a TV series with %d seasons", titleId, len(providerTitle.Seasons))
	}

	title := MapProviderTitleToDb(*providerTitle)

	// Set missing fields
	now := time.Now()
	title.AddedAt = &now
	title.UpdatedAt = &now

	if err := db.AddTitle(ctx, title); err != nil {
		if !errors.Is(err, store.ErrDuplicatedRecord) {
			return Title{}, err
		}
		// If duplicate, read back the stored document
		if stored, gerr := db.GetTitleById(ctx, titleId); gerr == nil {
			title = stored
		}
	}

	return MapDbTitleToApiTitle(title), nil
}

func DeleteTitle(db store.Store, ctx context.Context, titleId string) error {
	_, err := db.DeleteTitle(ctx, titleId)
	if err != nil {
		return err
	}

	return nil
}

func GetTitleById(db store.Store, ctx context.Context, titleId string) (Title, error) {
	titleDb, err := db.GetTitleById(ctx, titleId)
	if err != nil {
		return Title{}, err
	}

	return MapDbTitleToApiTitle(titleDb), nil
}

// GetEpisodes returns the episodes stored on a title. Fetched separately from
// the main title/list payload so large episode arrays are loaded on demand.
func GetEpisodes(db store.Store, ctx context.Context, titleId string) ([]Episode, error) {
	titleDb, err := db.GetTitleById(ctx, titleId)
	if err != nil {
		return nil, err
	}
	return MapDbEpisodesToImdbEpisodes(titleDb.Episodes), nil
}

func SearchTitles(provider titleprovider.Provider, ctx context.Context, searchQuery string, limit int) ([]Title, error) {
	items, err := provider.SearchTitles(ctx, searchQuery, limit)
	if err != nil {
		return nil, err
	}
	return MapProviderSearchItemsToTitles(items), nil
}

// TitleExists reports whether a title with the given id exists. It is a thin
// service passthrough so handlers reach the DB only through the service layer.
func TitleExists(db store.Store, ctx context.Context, titleId string) (bool, error) {
	return db.TitleExists(ctx, titleId)
}
