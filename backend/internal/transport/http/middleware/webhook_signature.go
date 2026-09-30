package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/transport/http/respond"
)

// HeaderSignature carries the HMAC of the request body.
const HeaderSignature = "X-Signature"

// maxSignedBodyBytes bounds what is read before verification, so an unauthenticated
// caller cannot make the process buffer an arbitrary amount of data.
const maxSignedBodyBytes = 64 << 10 // 64 KiB

// signaturePrefix is the optional algorithm prefix, following the convention most
// payment providers use: "sha256=<hex>". A bare hex digest is accepted too.
const signaturePrefix = "sha256="

// WebhookSignature verifies that the request body was signed with the shared secret.
//
// The scheme is deliberately the simplest verifiable one:
//
//	X-Signature: sha256=<hex(HMAC_SHA256(WEBHOOK_SECRET, raw_request_body))>
//
// The spec does not define how the provider authenticates itself, so this is an
// assumption, documented in the README. It is optional on purpose: with WEBHOOK_SECRET
// unset, verification is skipped and a warning is logged at startup, so the endpoint
// can be exercised with plain curl during review. In a real deployment the secret
// would be required.
//
// Replay protection comes from idempotency rather than from a timestamp header:
// replaying a signed body re-sends the same event_id, and the unique index makes
// that a no-op. A timestamp would only help if it were part of the signed payload,
// which would mean inventing a scheme the provider has not agreed to.
func WebhookSignature(secret string, logger *slog.Logger) func(http.Handler) http.Handler {
	secret = strings.TrimSpace(secret)

	return func(next http.Handler) http.Handler {
		if secret == "" {
			if logger != nil {
				logger.Warn("webhook signature verification is disabled because WEBHOOK_SECRET is empty")
			}
			return next
		}

		key := []byte(secret)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, maxSignedBodyBytes))
			if err != nil {
				respond.Error(w, r, paymentref.ErrInvalidWebhookPayload.WithDetail("body", "could not be read"))
				return
			}
			// The body was consumed to verify it, so hand the handler a fresh reader.
			r.Body = io.NopCloser(bytes.NewReader(body))

			if !validSignature(key, body, r.Header.Get(HeaderSignature)) {
				respond.Error(w, r, paymentref.ErrInvalidSignature)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validSignature(key, body []byte, header string) bool {
	provided := strings.TrimSpace(header)
	if provided == "" {
		return false
	}
	provided = strings.TrimPrefix(strings.ToLower(provided), signaturePrefix)

	decoded, err := hex.DecodeString(provided)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(body)

	// Constant-time comparison: a byte-by-byte one leaks how much of the digest was
	// correct, which is enough to forge a signature given enough attempts.
	return hmac.Equal(decoded, mac.Sum(nil))
}
