package factory

import (
	"fmt"
	"os"

	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/hybrid"
	"github.com/lealre/movies-backend/internal/titleprovider/imdbapi"
	"github.com/lealre/movies-backend/internal/titleprovider/omdb"
	"github.com/lealre/movies-backend/internal/titleprovider/tmdb"
)

// NewFromEnv builds the provider selected by the TITLE_PROVIDER env var.
// Allowed values (see internal/titleprovider/README.md for a comparison):
//   - "hybrid"  : TMDB metadata + OMDb IMDb ratings (recommended; needs both keys)
//   - "tmdb"    : TMDB only (default; rich seasons/episodes, TMDB's own ratings)
//   - "omdb"    : OMDb only (real IMDb ratings + Metacritic, thinner episode data)
//   - "imdbapi" : legacy api.imdbapi.dev (offline; kept for port-back)
func NewFromEnv() (titleprovider.Provider, error) {
	name := os.Getenv("TITLE_PROVIDER")
	if name == "" {
		name = "tmdb"
	}

	switch name {
	case "hybrid":
		tmdbKey := os.Getenv("TMDB_API_KEY")
		if tmdbKey == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=hybrid requires TMDB_API_KEY to be set")
		}
		omdbKey := os.Getenv("OMDB_API_KEY")
		if omdbKey == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=hybrid requires OMDB_API_KEY to be set")
		}
		return hybrid.New(tmdb.New(tmdbKey), omdbKey), nil
	case "tmdb":
		key := os.Getenv("TMDB_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=tmdb requires TMDB_API_KEY to be set")
		}
		return tmdb.New(key), nil
	case "omdb":
		key := os.Getenv("OMDB_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=omdb requires OMDB_API_KEY to be set")
		}
		return omdb.New(key), nil
	case "imdbapi":
		return imdbapi.New(), nil
	default:
		return nil, fmt.Errorf("unknown TITLE_PROVIDER %q (allowed: hybrid, tmdb, omdb, imdbapi)", name)
	}
}
