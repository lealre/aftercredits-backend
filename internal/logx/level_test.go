package logx

import (
	"log/slog"
	"testing"
)

// Mirrors TestActivityRetentionEnabledParsing in cmd/routines, and for the same
// reason: a typo in a deploy env must not silently change behaviour. Here the
// safe fallback is info — quiet enough for production, loud enough to be useful.
func TestLevelFromEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"  debug  ", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},

		{"", slog.LevelInfo},
		{"trace", slog.LevelInfo},
		{"dbug", slog.LevelInfo},
		{"verbose", slog.LevelInfo},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.value)
			if got := LevelFromEnv(); got != tc.want {
				t.Errorf("LevelFromEnv() with %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
