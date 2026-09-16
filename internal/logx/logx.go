package logx

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	loggerKey ctxKey = "logx.logger"
	traceKey  ctxKey = "logx.trace"
	userKey   ctxKey = "logx.user"
	ipKey     ctxKey = "logx.ip"
	methodKey ctxKey = "logx.method"
	pathKey   ctxKey = "logx.path"
)

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext returns the request's logger, or the process default when there
// is none — a background goroutine or a test still logs rather than panicking.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithRequest records the fields known when a request arrives. The handler
// reads them at emit time, so every record made during the request carries
// them without any call site passing them.
func WithRequest(ctx context.Context, trace, ip, method, path string) context.Context {
	ctx = context.WithValue(ctx, traceKey, trace)
	ctx = context.WithValue(ctx, ipKey, ip)
	ctx = context.WithValue(ctx, methodKey, method)
	return context.WithValue(ctx, pathKey, path)
}

// WithUser records the authenticated user id. AuthMiddleware calls this after
// the logger already exists — which is exactly why the handler reads context
// rather than binding its fields at construction.
func WithUser(ctx context.Context, userId string) context.Context {
	return context.WithValue(ctx, userKey, userId)
}

func fromCtx(ctx context.Context, key ctxKey) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
