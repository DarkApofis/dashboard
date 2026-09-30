// Package reqctx carries per-request values (request id, request-scoped logger)
// in the context. It is a leaf package so that middlewares and response helpers
// can both use it without importing each other.
package reqctx

import (
	"context"
	"log/slog"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	loggerKey
)

// WithRequestID stores the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request id, or an empty string outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithLogger stores a logger already decorated with the request attributes.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger returns the request-scoped logger, falling back to the default one so
// callers never need a nil check.
func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
