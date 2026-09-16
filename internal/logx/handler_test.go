package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func newTestLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(NewHandler(buf, level))
}

// The subtle property the whole design rests on: the logger is built when the
// request arrives, but the user id is not known until AuthMiddleware has run.
// A handler that bound its fields at construction could never carry it. If
// this regresses, user ids silently vanish from every log line and no test
// other than this one fails.
func TestHandlerReadsUserFromContextAtEmitTime(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, slog.LevelInfo)

	ctx := WithRequest(context.Background(), "a1b2c", "10.0.0.1", "GET", "/groups/5")
	logger.InfoContext(ctx, "before auth")

	ctx = WithUser(ctx, "9f3e2b81")
	logger.InfoContext(ctx, "after auth")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "user=-") {
		t.Errorf("pre-auth line must render user=-, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "user=9f3e2b81") {
		t.Errorf("post-auth line must carry the user id, got %q", lines[1])
	}
}

// Pins the user's stated field order against future handler edits.
func TestHandlerFieldOrder(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, slog.LevelInfo)

	ctx := WithUser(WithRequest(context.Background(), "a1b2c", "10.0.0.1", "GET", "/groups/5"), "9f3e")
	logger.InfoContext(ctx, "request completed", "status", 200, "dur_ms", 43)

	line := buf.String()
	order := []string{"trace=a1b2c", "user=9f3e", "ip=10.0.0.1", "GET", "/groups/5", "INFO", "request completed", "status=200", "dur_ms=43"}
	prev := -1
	for _, want := range order {
		at := strings.Index(line, want)
		if at < 0 {
			t.Fatalf("missing %q in %q", want, line)
		}
		if at < prev {
			t.Errorf("%q appears out of order in %q", want, line)
		}
		prev = at
	}
}

func TestHandlerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, slog.LevelInfo)

	logger.Debug("narration")
	if buf.Len() != 0 {
		t.Errorf("debug must be suppressed at info level, got %q", buf.String())
	}
	logger.Error("boom")
	if !strings.Contains(buf.String(), "ERROR") {
		t.Errorf("error must be emitted at info level, got %q", buf.String())
	}
}

// The path is attacker-controlled. Control bytes are already stripped upstream,
// but a value containing a space or '=' would still break the key=value shape
// and could forge a field. Quoting is this handler's job.
func TestHandlerQuotesAwkwardValues(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, slog.LevelInfo)

	logger.Info("odd", "note", "two words", "forged", "a=b")

	line := buf.String()
	if !strings.Contains(line, `note="two words"`) {
		t.Errorf("a value with a space must be quoted, got %q", line)
	}
	if !strings.Contains(line, `forged="a=b"`) {
		t.Errorf("a value containing = must be quoted, got %q", line)
	}
}

func TestHandlerWithoutRequestContext(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, slog.LevelInfo)

	logger.Info("startup")

	line := buf.String()
	if strings.Contains(line, "trace=") {
		t.Errorf("no request context means no trace field, got %q", line)
	}
	if !strings.Contains(line, "startup") {
		t.Errorf("message must still render, got %q", line)
	}
}
