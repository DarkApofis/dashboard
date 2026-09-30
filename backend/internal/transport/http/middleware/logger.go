package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"paymentlinks/internal/transport/http/reqctx"
)

// Logger writes one structured access line per request and, more importantly,
// puts a logger already tagged with the request id into the context, so every log
// emitted while serving the request is correlated without passing a logger around.
//
// It must run after RequestID.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger.With(
				slog.String("request_id", reqctx.RequestID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			ctx := reqctx.WithLogger(r.Context(), requestLogger)

			wrapped := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				level := slog.LevelInfo
				switch {
				case wrapped.Status() >= http.StatusInternalServerError:
					level = slog.LevelError
				case r.URL.Path == "/health":
					// Health is polled by orchestrators; at info it would drown
					// out everything else.
					level = slog.LevelDebug
				}

				requestLogger.LogAttrs(ctx, level, "http_request",
					slog.Int("status", wrapped.Status()),
					slog.Int("bytes", wrapped.BytesWritten()),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
					slog.String("remote_addr", r.RemoteAddr),
				)
			}()

			next.ServeHTTP(wrapped, r.WithContext(ctx))
		})
	}
}
