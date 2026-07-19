// internal/titleprovider/provider_test.go
package titleprovider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

// fakeProvider verifies the interface is implementable with the expected signatures.
type fakeProvider struct{}

func (fakeProvider) GetTitle(_ context.Context, imdbID string) (*titleprovider.Title, error) {
	if imdbID == "missing" {
		return nil, titleprovider.ErrTitleNotFound
	}
	return &titleprovider.Title{ID: imdbID, Type: "movie", PrimaryTitle: "X"}, nil
}
func (fakeProvider) SearchTitles(_ context.Context, _ string, _ int) ([]titleprovider.SearchItem, error) {
	return []titleprovider.SearchItem{{ID: "tt1", PrimaryTitle: "X"}}, nil
}
func (fakeProvider) Name() string { return "fake" }

func TestProviderInterfaceShape(t *testing.T) {
	var p titleprovider.Provider = fakeProvider{}

	got, err := p.GetTitle(context.Background(), "tt0068646")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "tt0068646" {
		t.Fatalf("got ID %q", got.ID)
	}

	_, err = p.GetTitle(context.Background(), "missing")
	if !errors.Is(err, titleprovider.ErrTitleNotFound) {
		t.Fatalf("expected ErrTitleNotFound, got %v", err)
	}

	items, err := p.SearchTitles(context.Background(), "x", 5)
	if err != nil || len(items) != 1 {
		t.Fatalf("search failed: %v items=%d", err, len(items))
	}
	if p.Name() != "fake" {
		t.Fatalf("name %q", p.Name())
	}
}
