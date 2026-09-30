package middleware

import (
	"net/http"
	"strings"
)

// allowedRequestHeaders are the headers the dashboard and the provider may send.
// X-Signature is there for the signed webhook.
const allowedRequestHeaders = "Content-Type, Accept, X-Request-ID, X-Signature"

const allowedMethods = "GET, POST, OPTIONS"

// CORS allows the browser dashboard to call the API from another origin.
//
// origin comes from CORS_ORIGIN and may be a comma-separated list, or "*". The
// matching origin is echoed back rather than reflecting whatever arrives, so an
// arbitrary site cannot talk to the API. An empty configuration disables CORS.
func CORS(origin string) func(http.Handler) http.Handler {
	allowed := parseOrigins(origin)

	return func(next http.Handler) http.Handler {
		if len(allowed) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestOrigin := r.Header.Get("Origin")
			if allowedOrigin, ok := resolveOrigin(allowed, requestOrigin); ok {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedRequestHeaders)
				w.Header().Set("Access-Control-Expose-Headers", HeaderRequestID)
				w.Header().Set("Access-Control-Max-Age", "600")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func resolveOrigin(allowed []string, requestOrigin string) (string, bool) {
	for _, candidate := range allowed {
		if candidate == "*" {
			return "*", true
		}
		if requestOrigin != "" && strings.EqualFold(candidate, requestOrigin) {
			return candidate, true
		}
	}
	return "", false
}
