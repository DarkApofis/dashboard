package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"paymentlinks/internal/domain/errs"
	"paymentlinks/internal/transport/http/reqctx"
	"paymentlinks/internal/transport/http/respond"
)

// Recover turns a panic into a 500 in the standard error envelope, logging the
// stack trace. Without it a panic kills the connection and the client gets nothing
// it can act on.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			// http.ErrAbortHandler is the documented way to abort a response on
			// purpose; it must keep propagating.
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			reqctx.Logger(r.Context()).Error("panic recovered",
				slog.Any("panic", recovered),
				slog.String("stack", string(debug.Stack())),
			)

			respond.Error(w, r, errs.Internal("internal_error", "unexpected internal error"))
		}()

		next.ServeHTTP(w, r)
	})
}
