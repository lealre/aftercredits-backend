package main

import "testing"

// TestActivityRetentionDefaultsToDisabled is the one that matters.
//
// Pruning used to run on every invocation of this job with a 90-day window and
// no way to switch it off. Nothing had aged past that window yet, so the
// behaviour was invisible — the first sign of it would have been events
// disappearing three months after the feed shipped. The default is now "keep
// everything", and this test exists so that flipping back is a decision someone
// has to argue with a failing test, not a one-word edit nobody notices.
func TestActivityRetentionDefaultsToDisabled(t *testing.T) {
	t.Setenv("ACTIVITY_RETENTION_ENABLED", "")

	if activityRetentionEnabled() {
		t.Fatal("activity retention must be OFF unless explicitly enabled; an append-only log should not lose rows by default")
	}
}

func TestActivityRetentionEnabledParsing(t *testing.T) {
	// Both directions are covered deliberately. Accepting several spellings of
	// "on" is a convenience; accepting anything unrecognised would be a defect,
	// because a typo in a deploy env would silently start deleting history.
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"  true  ", true},
		{"1", true},
		{"yes", true},
		{"on", true},

		{"", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"ture", false},  // a typo must not enable deletion
		{"maybe", false}, // nor must anything else unrecognised
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("ACTIVITY_RETENTION_ENABLED", tc.value)

			if got := activityRetentionEnabled(); got != tc.want {
				t.Errorf("activityRetentionEnabled() with %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// TestActivityRetentionDaysUnchanged pins the window's behaviour, which this
// change deliberately leaves alone — it only ever applies once retention is
// switched on.
func TestActivityRetentionDays(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{"unset falls back to 90", "", 90},
		{"a valid override is used", "30", 30},
		{"zero is rejected, not treated as 'delete everything'", "0", 90},
		{"a negative value is rejected", "-7", 90},
		{"garbage falls back rather than erroring", "soon", 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ACTIVITY_RETENTION_DAYS", tc.value)

			if got := activityRetentionDays(); got != tc.want {
				t.Errorf("activityRetentionDays() with %q = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}
