// Package validate holds the length ceilings for user-controlled text and small
// helpers to check them. The same limits are enforced at the database with
// CHECK constraints (migration 010); these are the first line that turns an
// over-long value into a clean 400 instead of a constraint-violation 500.
//
// Limits are counted in runes (user-visible characters), matching the DB's
// char_length(). The service layer owns the error vocabulary (CONVENTIONS §3);
// this package only answers "is this too long / does it contain control
// characters", so every domain can share one definition of the caps.
package validate

import "unicode/utf8"

// Field length ceilings, in runes.
const (
	NameMax        = 64
	EmailMax       = 254
	UsernameMax    = 32
	UsernameMin    = 3
	GroupNameMax   = 64
	GroupDescMax   = 500
	CommentMax     = 2000
	SearchQueryMax = 200
)

// TooLong reports whether s exceeds max runes.
func TooLong(s string, max int) bool {
	return utf8.RuneCountInString(s) > max
}

// HasControlChars reports whether s contains any control character other than
// the ordinary whitespace a comment legitimately uses (tab, newline, carriage
// return). It rejects NUL and the C0/C1 control range, which have no place in a
// username, name, group name or comment and are a common injection primitive.
func HasControlChars(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return true
		}
	}
	return false
}
