package hybrid

import (
	"context"
	"errors"
	"testing"

	"github.com/lealre/movies-backend/internal/titleprovider"
)

// fakeMeta stands in for the metadata provider (TMDB).
type fakeMeta struct {
	title  *titleprovider.Title
	search []titleprovider.SearchItem
	err    error
}

func (f fakeMeta) GetTitle(_ context.Context, imdbID string) (*titleprovider.Title, error) {
	if f.err != nil {
		return nil, f.err
	}
	cp := *f.title
	cp.ID = imdbID
	return &cp, nil
}
func (f fakeMeta) SearchTitles(_ context.Context, _ string, _ int) ([]titleprovider.SearchItem, error) {
	return f.search, nil
}
func (f fakeMeta) Name() string { return "fakeMeta" }

// fakeRater stands in for OMDb's RatingByID.
type fakeRater struct {
	rating titleprovider.Rating
	meta   *titleprovider.Metacritic
	err    error
}

func (f fakeRater) RatingByID(_ context.Context, _ string) (titleprovider.Rating, *titleprovider.Metacritic, error) {
	return f.rating, f.meta, f.err
}

func baseTitle() *titleprovider.Title {
	return &titleprovider.Title{
		Type:         "movie",
		PrimaryTitle: "Michael",
		Rating:       titleprovider.Rating{AggregateRating: 8.6, VoteCount: 3680}, // TMDB's rating
	}
}

func TestGetTitle_OverlaysImdbRating(t *testing.T) {
	meta := fakeMeta{title: baseTitle()}
	rater := fakeRater{rating: titleprovider.Rating{AggregateRating: 7.4, VoteCount: 157000}, meta: &titleprovider.Metacritic{Score: 55}}
	p := newWith(meta, rater)

	got, err := p.GetTitle(context.Background(), "tt11378946")
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	// Metadata preserved from meta provider...
	if got.PrimaryTitle != "Michael" || got.Type != "movie" {
		t.Fatalf("metadata not preserved: %+v", got)
	}
	// ...but the rating is now the IMDb one from OMDb.
	if got.Rating.AggregateRating != 7.4 || got.Rating.VoteCount != 157000 {
		t.Fatalf("rating not overlaid: %+v (want 7.4/157000)", got.Rating)
	}
	if got.Metacritic == nil || got.Metacritic.Score != 55 {
		t.Fatalf("metacritic not overlaid: %+v", got.Metacritic)
	}
}

func TestGetTitle_RaterErrorKeepsMetaRating(t *testing.T) {
	meta := fakeMeta{title: baseTitle()}
	rater := fakeRater{err: errors.New("omdb down")}
	p := newWith(meta, rater)

	got, err := p.GetTitle(context.Background(), "tt11378946")
	if err != nil {
		t.Fatalf("hybrid must not fail when the rating source errors: %v", err)
	}
	if got.Rating.AggregateRating != 8.6 { // fell back to meta's rating
		t.Fatalf("expected meta rating 8.6 on rater error, got %v", got.Rating.AggregateRating)
	}
}

func TestGetTitle_ZeroImdbRatingKeepsMeta(t *testing.T) {
	meta := fakeMeta{title: baseTitle()}
	rater := fakeRater{rating: titleprovider.Rating{AggregateRating: 0}} // OMDb had no rating (N/A)
	p := newWith(meta, rater)

	got, _ := p.GetTitle(context.Background(), "tt11378946")
	if got.Rating.AggregateRating != 8.6 {
		t.Fatalf("expected meta rating kept when OMDb rating is 0, got %v", got.Rating.AggregateRating)
	}
}

func TestGetTitle_MetaErrorPropagates(t *testing.T) {
	meta := fakeMeta{err: errors.New("tmdb boom")}
	p := newWith(meta, fakeRater{})
	if _, err := p.GetTitle(context.Background(), "tt1"); err == nil {
		t.Fatal("expected metadata provider error to propagate")
	}
}

func TestSearchDelegatesToMeta(t *testing.T) {
	meta := fakeMeta{search: []titleprovider.SearchItem{{ID: "tt0068646", PrimaryTitle: "The Godfather"}}}
	p := newWith(meta, fakeRater{})
	items, err := p.SearchTitles(context.Background(), "godfather", 5)
	if err != nil || len(items) != 1 || items[0].ID != "tt0068646" {
		t.Fatalf("search delegation failed: %v %+v", err, items)
	}
}

func TestName(t *testing.T) {
	if newWith(fakeMeta{}, fakeRater{}).Name() != "hybrid" {
		t.Fatal("name should be hybrid")
	}
}
