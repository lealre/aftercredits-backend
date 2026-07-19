package factory

import (
	"fmt"
	"os"

	"github.com/lealre/movies-backend/internal/titleprovider"
	"github.com/lealre/movies-backend/internal/titleprovider/imdbapi"
	"github.com/lealre/movies-backend/internal/titleprovider/tmdb"
)

// NewFromEnv builds the provider selected by the TITLE_PROVIDER env var.
// Allowed values: "tmdb" (default), "imdbapi".
func NewFromEnv() (titleprovider.Provider, error) {
	name := os.Getenv("TITLE_PROVIDER")
	if name == "" {
		name = "tmdb"
	}

	switch name {
	case "tmdb":
		key := os.Getenv("TMDB_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TITLE_PROVIDER=tmdb requires TMDB_API_KEY to be set")
		}
		return tmdb.New(key), nil
	case "imdbapi":
		return imdbapi.New(), nil
	default:
		return nil, fmt.Errorf("unknown TITLE_PROVIDER %q (allowed: tmdb, imdbapi)", name)
	}
}
