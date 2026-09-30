// Package middleware holds the HTTP middlewares: request id, access log, panic
// recovery and CORS. They are plain func(http.Handler) http.Handler values, so
// they work with the standard library and with any router.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"paymentlinks/internal/transport/http/reqctx"
)

// HeaderRequestID is the header the id travels in, both ways.
const HeaderRequestID = "X-Request-ID"

const maxRequestIDLength = 64

// RequestID assigns an identifier to every request, reusing the incoming one when
// a caller (a gateway, the frontend) already provides it so a single trace spans
// several services. The id is echoed back in the response header and attached to
// every log line of the request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get(HeaderRequestID))
		if id == "" {
			id = newRequestID()
		}

		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(reqctx.WithRequestID(r.Context(), id)))
	})
}

// sanitizeRequestID accepts only short, printable ASCII so a client cannot inject
// control characters into the response header or the logs.
func sanitizeRequestID(raw string) string {
	if raw == "" || len(raw) > maxRequestIDLength {
		return ""
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < 0x21 || raw[i] > 0x7e {
			return ""
		}
	}
	return raw
}

func newRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Randomness is unavailable: a timestamp still correlates the log lines
		// of one request, which is all the id is for.
		return "ts-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf[:])
}
