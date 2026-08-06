package pgmigration

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/lealre/movies-backend/internal/mongodb"
)

// Dump holds every document from a mongodump backup, decoded into the
// mongodb package's bson-tagged Db structs (ground truth for field names).
type Dump struct {
	Users    []mongodb.UserDb
	Titles   []mongodb.TitleDb
	Ratings  []mongodb.RatingDb
	Comments []mongodb.CommentDb
	Groups   []mongodb.GroupDb

	// Warnings collects non-fatal oddities found while reading: missing
	// collection files (treated as empty) and data-shape quirks that cannot
	// round-trip through the relational schema (present-but-empty season
	// maps, titles-map keys that disagree with the item's own titleId).
	Warnings []string
}

// maxBSONSize is the BSON spec's document size cap (16 MiB); a larger length
// prefix in a dump file means corruption.
const maxBSONSize = 16 * 1024 * 1024

// ReadDump reads an extracted `mongodump --out` backup. dir may be the
// database directory itself (containing *.bson) or its parent.
func ReadDump(dir string) (*Dump, error) {
	base, err := resolveDumpDir(dir)
	if err != nil {
		return nil, err
	}

	d := &Dump{}
	if err := readCollection(base, "users", &d.Users, &d.Warnings); err != nil {
		return nil, err
	}
	if err := readCollection(base, "titles", &d.Titles, &d.Warnings); err != nil {
		return nil, err
	}
	if err := readCollection(base, "ratings", &d.Ratings, &d.Warnings); err != nil {
		return nil, err
	}
	if err := readCollection(base, "comments", &d.Comments, &d.Warnings); err != nil {
		return nil, err
	}
	if err := readCollection(base, "groups", &d.Groups, &d.Warnings); err != nil {
		return nil, err
	}

	d.sanityCheck()
	return d, nil
}

// sanityCheck appends warnings for data shapes that cannot round-trip
// through the relational schema. Neither blocks the migration: an empty
// season map loads as zero child rows (and reads back as nil, the store's
// convention), and a titles-map entry loads under its map key.
func (d *Dump) sanityCheck() {
	for _, r := range d.Ratings {
		if r.SeasonsRatings != nil && len(*r.SeasonsRatings) == 0 {
			d.Warnings = append(d.Warnings,
				fmt.Sprintf("rating %s: seasonsRatings is present but empty; loads as no season rows", r.Id))
		}
	}
	for _, c := range d.Comments {
		if c.SeasonsComments != nil && len(*c.SeasonsComments) == 0 {
			d.Warnings = append(d.Warnings,
				fmt.Sprintf("comment %s: seasonsComments is present but empty; loads as no season rows", c.Id))
		}
	}
	for _, g := range d.Groups {
		for key, item := range g.Titles {
			if string(key) != item.TitleId {
				d.Warnings = append(d.Warnings,
					fmt.Sprintf("group %s: titles map key %q != item titleId %q; the map key wins", g.Id, key, item.TitleId))
			}
			if item.SeasonsWatched != nil && len(*item.SeasonsWatched) == 0 {
				d.Warnings = append(d.Warnings,
					fmt.Sprintf("group %s title %s: seasonsWatched is present but empty; loads as no season rows", g.Id, key))
			}
		}
	}
}

// resolveDumpDir accepts either the database directory itself or the
// mongodump --out parent: it tries dir, then dir/$MONGO_DB (default
// aftercreditsdb), then falls back to a single subdirectory holding .bson
// files. Anything else is an error.
func resolveDumpDir(dir string) (string, error) {
	if hasBSONFiles(dir) {
		return dir, nil
	}
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "aftercreditsdb"
	}
	if sub := filepath.Join(dir, dbName); hasBSONFiles(sub) {
		return sub, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dump dir: %w", err)
	}
	var candidates []string
	for _, e := range entries {
		if e.IsDir() && hasBSONFiles(filepath.Join(dir, e.Name())) {
			candidates = append(candidates, filepath.Join(dir, e.Name()))
		}
	}
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return "", fmt.Errorf("no .bson files found in %s (or a database subdirectory of it)", dir)
	default:
		return "", fmt.Errorf("multiple database subdirectories with .bson files in %s: %v — pass the database directory directly", dir, candidates)
	}
}

func hasBSONFiles(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.bson"))
	return len(matches) > 0
}

// readCollection decodes <base>/<name>.bson into out. A missing file is a
// warning (empty collection); any decode error is fatal.
func readCollection[T any](base, name string, out *[]T, warnings *[]string) error {
	path := filepath.Join(base, name+".bson")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		*warnings = append(*warnings,
			fmt.Sprintf("collection file %s not found — treating %s as empty", path, name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	docs, err := decodeBSONStream[T](f)
	if err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	*out = docs
	return nil
}

// decodeBSONStream reads mongodump's on-disk format: concatenated BSON
// documents, each starting with a little-endian int32 total length that
// includes the 4 length bytes themselves.
func decodeBSONStream[T any](r io.Reader) ([]T, error) {
	var out []T
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil // clean end of stream
			}
			return nil, fmt.Errorf("read document length: %w", err)
		}
		size := int32(binary.LittleEndian.Uint32(lenBuf[:]))
		if size < 5 || size > maxBSONSize {
			return nil, fmt.Errorf("invalid BSON document length %d", size)
		}
		raw := make([]byte, size)
		copy(raw, lenBuf[:])
		if _, err := io.ReadFull(r, raw[4:]); err != nil {
			return nil, fmt.Errorf("read document body (truncated file?): %w", err)
		}
		var doc T
		if err := bson.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("unmarshal document: %w", err)
		}
		out = append(out, doc)
	}
}
