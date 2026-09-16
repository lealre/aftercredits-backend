// Package logx carries a per-request logger through context and renders it.
//
// The rendering is a custom slog.Handler rather than slog.TextHandler for two
// reasons. TextHandler's field order is fixed (time, level, msg, then attrs)
// and the agreed format puts the request's common fields BEFORE the level, so
// a line reads "who and where" before "how bad". And the common fields are not
// attributes on the record at all — they are read from the context at emit
// time, which is what lets a logger created before authentication still carry
// the user id afterwards.
package logx

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

type Handler struct {
	mu    *sync.Mutex
	out   io.Writer
	level slog.Leveler
	attrs []slog.Attr
}

func NewHandler(w io.Writer, level slog.Leveler) *Handler {
	return &Handler{mu: &sync.Mutex{}, out: w, level: level}
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

// WithAttrs shares the mutex and writer with its parent on purpose: every
// derived logger must serialise against the same output, or concurrent
// requests interleave mid-line.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

// Groups are not used by this codebase, and nesting would complicate the flat
// key=value contract the format promises. A group is a no-op rather than a
// silent structural change to the output.
func (h *Handler) WithGroup(string) slog.Handler { return h }

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	b.WriteString(r.Time.Format("15:04:05"))

	// Common fields, from context, in the agreed order. A request-scoped record
	// has all of them; a startup or background record has none and omits the
	// whole prefix rather than rendering empty keys.
	if trace := fromCtx(ctx, traceKey); trace != "" {
		writeField(&b, "trace", trace)
		user := fromCtx(ctx, userKey)
		if user == "" {
			user = "-"
		}
		writeField(&b, "user", user)
		writeField(&b, "ip", fromCtx(ctx, ipKey))
		// Method and path render bare rather than as key=value: they are the
		// one pair that reads better as prose, and they are always adjacent.
		b.WriteByte(' ')
		b.WriteString(fromCtx(ctx, methodKey))
		b.WriteByte(' ')
		b.WriteString(fromCtx(ctx, pathKey))
	}

	b.WriteString("  ")
	b.WriteString(r.Level.String())
	b.WriteString("  ")
	b.WriteString(r.Message)

	for _, a := range h.attrs {
		writeAttr(&b, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, a)
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

func writeAttr(b *strings.Builder, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	writeField(b, a.Key, a.Value.Resolve().String())
}

// writeField quotes any value that would otherwise break the key=value shape.
// Without this, a value containing a space or '=' could forge a field — and
// the request path, which appears in these lines, is attacker-controlled.
func writeField(b *strings.Builder, key, value string) {
	b.WriteByte(' ')
	b.WriteString(key)
	b.WriteByte('=')
	if value == "" || strings.ContainsAny(value, " =\"") {
		b.WriteString(strconv.Quote(value))
		return
	}
	b.WriteString(value)
}
