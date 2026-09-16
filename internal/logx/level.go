package logx

import (
	"log/slog"
	"os"
	"strings"
)

// LevelFromEnv reads LOG_LEVEL, defaulting to info.
//
// An unrecognised value falls back rather than failing: a mistyped level in a
// deploy env should not take the server down, and should not silently make it
// quieter than the operator expects either. Info is the safe middle.
func LevelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
