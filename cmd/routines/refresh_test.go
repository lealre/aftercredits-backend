package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/models"
	"github.com/lealre/movies-backend/internal/titleprovider"
)

func TestRefreshTitle(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	base := models.Title{
		ID: "tt1", Type: "movie", PrimaryTitle: "Kept",
		PrimaryImage: models.Image{URL: "old.png", Width: 1, Height: 1},
		Rating:       models.Rating{AggregateRating: 7.0, VoteCount: 10},
	}

	t.Run("no content change still stamps UpdatedAt", func(t *testing.T) {
		api := &titleprovider.Title{
			PrimaryImage: titleprovider.Image{URL: "old.png", Width: 1, Height: 1},
			Rating:       titleprovider.Rating{AggregateRating: 7.0, VoteCount: 10},
		}
		got, changed := refreshTitle(base, api, now)
		assert.False(t, changed)
		require.NotNil(t, got.UpdatedAt)
		assert.True(t, got.UpdatedAt.Equal(now))
		assert.Equal(t, "Kept", got.PrimaryTitle)
	})

	t.Run("rating and image changes are applied", func(t *testing.T) {
		api := &titleprovider.Title{
			PrimaryImage: titleprovider.Image{URL: "new.png", Width: 2, Height: 2},
			Rating:       titleprovider.Rating{AggregateRating: 8.5, VoteCount: 99},
			Metacritic:   &titleprovider.Metacritic{Score: 80, ReviewCount: 5},
		}
		got, changed := refreshTitle(base, api, now)
		assert.True(t, changed)
		assert.Equal(t, "new.png", got.PrimaryImage.URL)
		assert.Equal(t, 8.5, got.Rating.AggregateRating)
		require.NotNil(t, got.Metacritic)
		assert.Equal(t, 80, got.Metacritic.Score)
	})

	t.Run("untouched fields survive", func(t *testing.T) {
		api := &titleprovider.Title{
			PrimaryImage: titleprovider.Image{URL: "old.png", Width: 1, Height: 1},
			Rating:       titleprovider.Rating{AggregateRating: 7.0, VoteCount: 10},
		}
		withExtras := base
		withExtras.Plot = "a plot"
		withExtras.Genres = []string{"Drama"}
		got, _ := refreshTitle(withExtras, api, now)
		assert.Equal(t, "a plot", got.Plot)
		assert.Equal(t, []string{"Drama"}, got.Genres)
	})
}
