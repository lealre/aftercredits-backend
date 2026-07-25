// Package hybrid composes two providers: a metadata provider (TMDB — good at
// titles, seasons, episodes, images) and a rating source (OMDb — the real IMDb
// rating). GetTitle takes all metadata from the metadata provider, then overlays
// the authoritative IMDb rating (and Metacritic) from OMDb. See ../README.md.
package hybrid

import (
	"context"

	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/omdb"
)

// RatingSource supplies an IMDb rating for a title by IMDb ID. Implemented by
// *omdb.Provider (via RatingByID); an interface here keeps hybrid testable.
type RatingSource interface {
	RatingByID(ctx context.Context, imdbID string) (titleprovider.Rating, *titleprovider.Metacritic, error)
}

// Provider implements titleprovider.Provider by combining a metadata provider
// with a rating source.
type Provider struct {
	meta  titleprovider.Provider
	rater RatingSource
}

// New builds a hybrid provider from a metadata provider (e.g. TMDB) and an OMDb
// API key used to fetch IMDb ratings.
func New(meta titleprovider.Provider, omdbAPIKey string) *Provider {
	return &Provider{meta: meta, rater: omdb.New(omdbAPIKey)}
}

// newWith is the test seam; it injects an arbitrary metadata provider and rating source.
func newWith(meta titleprovider.Provider, rater RatingSource) *Provider {
	return &Provider{meta: meta, rater: rater}
}

func (p *Provider) Name() string { return "hybrid" }

// GetTitle fetches metadata from the metadata provider, then overlays the IMDb
// rating (and Metacritic) from the rating source. If the rating source fails or
// has no rating, the metadata provider's rating is kept (graceful degradation),
// so a momentary OMDb outage never blocks adding or refreshing a title.
func (p *Provider) GetTitle(ctx context.Context, imdbID string) (*titleprovider.Title, error) {
	t, err := p.meta.GetTitle(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	rating, metacritic, rerr := p.rater.RatingByID(ctx, imdbID)
	if rerr == nil {
		if rating.AggregateRating > 0 {
			t.Rating = rating
		}
		if metacritic != nil {
			t.Metacritic = metacritic
		}
	}
	return t, nil
}

// SearchTitles delegates to the metadata provider. Search results therefore
// carry the metadata provider's rating (used only to preview candidates); the
// authoritative IMDb rating is applied by GetTitle when a title is added.
func (p *Provider) SearchTitles(ctx context.Context, query string, limit int) ([]titleprovider.SearchItem, error) {
	return p.meta.SearchTitles(ctx, query, limit)
}
