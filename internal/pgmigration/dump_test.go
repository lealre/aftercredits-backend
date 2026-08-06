package pgmigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lealre/movies-backend/internal/mongodb"
)

func TestReadDump_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeDumpFiles(t, dir, fixtureDump())

	got, err := ReadDump(dir)
	require.NoError(t, err)

	assert.Len(t, got.Users, 2)
	assert.Len(t, got.Titles, 2)
	assert.Len(t, got.Ratings, 2)
	assert.Len(t, got.Comments, 2)
	assert.Len(t, got.Groups, 2)
	assert.Empty(t, got.Warnings)

	// Spot-check decoded values. BSON decodes datetimes into Local-location
	// time.Time values — compare instants with .Equal, never == / DeepEqual.
	u := got.Users[0]
	assert.Equal(t, fixU1, u.Id)
	require.NotNil(t, u.AvatarURL)
	assert.True(t, u.CreatedAt.Equal(ts(0)))
	require.NotNil(t, u.LastLoginAt)
	assert.True(t, u.LastLoginAt.Equal(ts(10)))

	series := got.Titles[1]
	assert.Equal(t, fixT2, series.ID)
	assert.Len(t, series.Seasons, 2)
	assert.Len(t, series.Episodes, 1)

	r2 := got.Ratings[1]
	require.NotNil(t, r2.SeasonsRatings)
	assert.Equal(t, float32(7.5), (*r2.SeasonsRatings)["1"].Rating)

	g2 := got.Groups[1]
	assert.True(t, g2.Deleted)
	require.NotNil(t, g2.DeletedAt)
	assert.True(t, g2.DeletedAt.Equal(ts(50)))
}

func TestReadDump_ResolvesDbSubdirectory(t *testing.T) {
	parent := t.TempDir()
	sub := filepath.Join(parent, "aftercreditsdb")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	writeDumpFiles(t, sub, fixtureDump())

	got, err := ReadDump(parent)
	require.NoError(t, err)
	assert.Len(t, got.Users, 2)
}

func TestReadDump_MissingCollectionWarns(t *testing.T) {
	dir := t.TempDir()
	d := fixtureDump()
	writeCollection(t, dir, "users", d.Users) // only users.bson exists

	got, err := ReadDump(dir)
	require.NoError(t, err)
	assert.Len(t, got.Users, 2)
	assert.Empty(t, got.Titles)
	require.NotEmpty(t, got.Warnings)
	joined := strings.Join(got.Warnings, "\n")
	for _, name := range []string{"titles", "ratings", "comments", "groups"} {
		assert.Contains(t, joined, name)
	}
}

func TestReadDump_EmptyFileIsEmptyCollection(t *testing.T) {
	dir := t.TempDir()
	d := fixtureDump()
	writeDumpFiles(t, dir, d)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "titles.bson"), nil, 0o644))

	got, err := ReadDump(dir)
	require.NoError(t, err)
	assert.Empty(t, got.Titles)
	assert.Len(t, got.Users, 2)
}

func TestReadDump_TruncatedFileFails(t *testing.T) {
	dir := t.TempDir()
	d := fixtureDump()
	writeDumpFiles(t, dir, d)

	raw, err := os.ReadFile(filepath.Join(dir, "users.bson"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "users.bson"), raw[:len(raw)-10], 0o644))

	_, err = ReadDump(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "users")
}

func TestReadDump_NoBSONAnywhereFails(t *testing.T) {
	_, err := ReadDump(t.TempDir())
	require.Error(t, err)
}

func TestReadDump_WarnsOnEmptySeasonMaps(t *testing.T) {
	dir := t.TempDir()
	d := fixtureDump()
	empty := make(mongodb.SeasonsRatingsDb)
	d.Ratings[0].SeasonsRatings = &empty // present-but-empty map
	writeDumpFiles(t, dir, d)

	got, err := ReadDump(dir)
	require.NoError(t, err)
	require.NotEmpty(t, got.Warnings)
	assert.Contains(t, strings.Join(got.Warnings, "\n"), fixR1)
}
