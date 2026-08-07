package pgmigration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/lealre/movies-backend/internal/mongodb"
)

// Fixture ids, shared across tests so assertions can reference them.
const (
	fixU1 = "64b0000000000000000000a1" // Alice — owner of both groups
	fixU2 = "64b0000000000000000000a2" // Bob
	fixG1 = "64b0000000000000000000c1" // live group
	fixG2 = "64b0000000000000000000c2" // soft-deleted group
	fixR1 = "64b0000000000000000000b1" // movie rating (no seasons)
	fixR2 = "64b0000000000000000000b2" // series rating (seasonsRatings)
	fixC1 = "64b0000000000000000000d1" // movie comment (top-level text)
	fixC2 = "64b0000000000000000000d2" // series comment (seasonsComments)
	fixT1 = "tt0000001"                // movie
	fixT2 = "tt0000002"                // tvSeries
)

func ts(min int) time.Time {
	return time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)
}

func tsPtr(min int) *time.Time { v := ts(min); return &v }

func strPtr(s string) *string { return &s }

// fixtureDump returns an in-memory dump exercising every fidelity path the
// migration must preserve. It is internally consistent (user.groups matches
// group.users for non-deleted groups) so a clean run produces no warnings.
func fixtureDump() *Dump {
	seasonsWatched := mongodb.SeasonWatchedDb{
		"1": {Watched: true, WatchedAt: tsPtr(40), AddedAt: ts(38), UpdatedAt: ts(40)},
		"2": {Watched: false, AddedAt: ts(39), UpdatedAt: ts(39)},
	}
	seasonsRatings := mongodb.SeasonsRatingsDb{
		"1": {Rating: 7.5, AddedAt: ts(45), UpdatedAt: ts(46)},
	}
	seasonsComments := mongodb.SeasonsCommentsDb{
		"1": {Comment: "good season", AddedAt: ts(47), UpdatedAt: ts(48)},
	}

	return &Dump{
		Users: []mongodb.UserDb{
			{
				Id: fixU1, Name: "Alice", Email: "alice@example.com", Username: "alice",
				PasswordHash: "hash-a", AvatarURL: strPtr("https://example.com/a.png"),
				Groups: []string{fixG1}, Role: "admin", IsActive: true,
				LastLoginAt: tsPtr(10), CreatedAt: ts(0), UpdatedAt: ts(1),
			},
			{
				Id: fixU2, Name: "Bob", Email: "bob@example.com", Username: "bob",
				PasswordHash: "hash-b", Groups: []string{fixG1}, Role: "user",
				IsActive: true, CreatedAt: ts(2), UpdatedAt: ts(3),
			},
		},
		Titles: []mongodb.TitleDb{
			{
				ID: fixT1, Type: "movie", PrimaryTitle: "Fixture Movie",
				PrimaryImage: mongodb.Image{URL: "https://example.com/m.png", Width: 100, Height: 150},
				StartYear:    2019, RuntimeSeconds: 5400, Genres: []string{"Drama", "Comedy"},
				Rating:          mongodb.Rating{AggregateRating: 7.2, VoteCount: 500},
				Metacritic:      &mongodb.Metacritic{Score: 70, ReviewCount: 12},
				Plot:            "A movie fixture.",
				Directors:       []mongodb.Person{{ID: "nm0000001", DisplayName: "Dana Director"}},
				OriginCountries: []mongodb.CodeName{{Code: "US", Name: "United States"}},
				AddedAt:         tsPtr(15), UpdatedAt: tsPtr(16),
			},
			{
				ID: fixT2, Type: "tvSeries", PrimaryTitle: "Fixture Show",
				StartYear: 2020, Genres: []string{"Drama"},
				Rating:  mongodb.Rating{AggregateRating: 8.1, VoteCount: 1000},
				Plot:    "A series fixture.",
				Stars:   []mongodb.Person{{ID: "nm0000002", DisplayName: "Sam Star", PrimaryProfessions: []string{"actor"}}},
				Seasons: []mongodb.Seasons{{Season: "1", EpisodeCount: 2}, {Season: "2", EpisodeCount: 3}},
				Episodes: []mongodb.Episode{
					{ID: fixT2 + "e1", Title: "Pilot", Season: "1", EpisodeNumber: 1},
				},
				AddedAt: tsPtr(20), UpdatedAt: tsPtr(21),
			},
		},
		Ratings: []mongodb.RatingDb{
			{Id: fixR1, TitleId: fixT1, UserId: fixU1, Note: 8.5, CreatedAt: ts(44), UpdatedAt: ts(44)},
			{Id: fixR2, TitleId: fixT2, UserId: fixU2, Note: 7.5,
				SeasonsRatings: &seasonsRatings, CreatedAt: ts(45), UpdatedAt: ts(46)},
		},
		Comments: []mongodb.CommentDb{
			{Id: fixC1, TitleId: fixT1, UserId: fixU1, Comment: strPtr("loved it"),
				CreatedAt: ts(47), UpdatedAt: ts(47)},
			{Id: fixC2, TitleId: fixT2, UserId: fixU2,
				SeasonsComments: &seasonsComments, CreatedAt: ts(48), UpdatedAt: ts(49)},
		},
		Groups: []mongodb.GroupDb{
			{
				Id: fixG1, Name: "movie night", Description: "friends", OwnerId: fixU1,
				Users: mongodb.UsersIds{fixU1, fixU2},
				Titles: mongodb.GroupTitleDb{
					mongodb.TitleId(fixT1): {TitleId: fixT1, TitleType: "movie",
						Watched: true, WatchedAt: tsPtr(30), AddedAt: ts(29), UpdatedAt: ts(30)},
					mongodb.TitleId(fixT2): {TitleId: fixT2, TitleType: "tvSeries",
						SeasonsWatched: &seasonsWatched, Watched: true,
						WatchedAt: tsPtr(40), AddedAt: ts(37), UpdatedAt: ts(40)},
				},
				CreatedAt: ts(25), UpdatedAt: ts(40),
			},
			{
				// Soft-deleted: mongo keeps users[] but members' user.groups no
				// longer reference it (the service removed them on delete).
				Id: fixG2, Name: "old group", OwnerId: fixU1,
				Users:   mongodb.UsersIds{fixU1},
				Deleted: true, DeletedAt: tsPtr(50),
				CreatedAt: ts(24), UpdatedAt: ts(50),
			},
		},
	}
}

// writeCollection writes docs to <dir>/<name>.bson in mongodump's on-disk
// format: concatenated bson.Marshal outputs (bson.Marshal already emits the
// length-prefixed document).
func writeCollection[T any](t *testing.T, dir, name string, docs []T) {
	t.Helper()
	var buf bytes.Buffer
	for _, doc := range docs {
		raw, err := bson.Marshal(doc)
		if err != nil {
			t.Fatalf("marshal %s doc: %v", name, err)
		}
		buf.Write(raw)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".bson"), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s.bson: %v", name, err)
	}
}

// writeDumpFiles writes all five collections of d into dir.
func writeDumpFiles(t *testing.T, dir string, d *Dump) {
	t.Helper()
	writeCollection(t, dir, "users", d.Users)
	writeCollection(t, dir, "titles", d.Titles)
	writeCollection(t, dir, "ratings", d.Ratings)
	writeCollection(t, dir, "comments", d.Comments)
	writeCollection(t, dir, "groups", d.Groups)
}
