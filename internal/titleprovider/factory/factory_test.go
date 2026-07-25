package factory_test

import (
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider/factory"
)

func TestNewFromEnv(t *testing.T) {
	t.Run("defaults to tmdb", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "")
		t.Setenv("TMDB_API_KEY", "k")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "tmdb" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("tmdb requires key", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "tmdb")
		t.Setenv("TMDB_API_KEY", "")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error for missing TMDB_API_KEY")
		}
	})

	t.Run("imdbapi", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "imdbapi")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "imdbapi" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("omdb", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "omdb")
		t.Setenv("OMDB_API_KEY", "k")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "omdb" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("omdb requires key", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "omdb")
		t.Setenv("OMDB_API_KEY", "")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error for missing OMDB_API_KEY")
		}
	})

	t.Run("hybrid needs both keys", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "hybrid")
		t.Setenv("TMDB_API_KEY", "tk")
		t.Setenv("OMDB_API_KEY", "")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error when OMDB_API_KEY missing for hybrid")
		}
		t.Setenv("TMDB_API_KEY", "")
		t.Setenv("OMDB_API_KEY", "ok")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error when TMDB_API_KEY missing for hybrid")
		}
	})

	t.Run("hybrid", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "hybrid")
		t.Setenv("TMDB_API_KEY", "tk")
		t.Setenv("OMDB_API_KEY", "ok")
		p, err := factory.NewFromEnv()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if p.Name() != "hybrid" {
			t.Fatalf("name %q", p.Name())
		}
	})

	t.Run("unknown value errors", func(t *testing.T) {
		t.Setenv("TITLE_PROVIDER", "bogus")
		if _, err := factory.NewFromEnv(); err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}
