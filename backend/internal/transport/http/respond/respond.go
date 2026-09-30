// Package respond writes every HTTP response of the API and is the single place
// where a domain error becomes a status code. Handlers never build an error body
// by hand, so the contract cannot drift endpoint by endpoint.
package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"paymentlinks/internal/domain/errs"
	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/reqctx"
)

// kindStatus is the error mapping table: the only translation from domain
// vocabulary to HTTP vocabulary in the codebase.
var kindStatus = map[errs.Kind]int{
	errs.KindValidation:    http.StatusBadRequest,          // 400: unintelligible input
	errs.KindUnauthorized:  http.StatusUnauthorized,        // 401: bad webhook signature
	errs.KindNotFound:      http.StatusNotFound,            // 404: missing resource
	errs.KindConflict:      http.StatusConflict,            // 409: clashes with current state
	errs.KindUnprocessable: http.StatusUnprocessableEntity, // 422: valid shape, broken rule
	errs.KindInternal:      http.StatusInternalServerError,
}

// StatusFor returns the HTTP status for a domain error kind.
func StatusFor(kind errs.Kind) int {
	if status, ok := kindStatus[kind]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// JSON writes a JSON body with the given status. The payload is marshalled before
// the header is written, so a marshalling failure cannot produce a truncated body
// behind a 200.
func JSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if payload == nil || status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		reqctx.Logger(r.Context()).Error("could not marshal response body",
			slog.String("error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"unexpected internal error"}}`))
		return
	}

	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// Error writes err as the API error envelope.
//
// Anything that is not a domain error is reported as a generic 500: an unexpected
// failure must never leak an internal message to the client. The original cause is
// logged instead, correlated by request id.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	domainErr, ok := errs.From(err)
	if !ok {
		domainErr = errs.Internal("internal_error", "unexpected internal error").Wrap(err)
	}

	status := StatusFor(domainErr.Kind)

	logger := reqctx.Logger(r.Context())
	if status >= http.StatusInternalServerError {
		logger.Error("request failed",
			slog.String("code", domainErr.Code),
			slog.String("error", err.Error()),
		)
	} else {
		logger.Debug("request rejected",
			slog.Int("status", status),
			slog.String("code", domainErr.Code),
			slog.String("error", err.Error()),
		)
	}

	JSON(w, r, status, dto.NewErrorEnvelope(domainErr.Code, domainErr.Message, domainErr.Details))
}

// ErrorStatus writes a transport-level error, the kind that never reaches the
// domain: a malformed body, an unknown route, a rejected signature.
func ErrorStatus(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, r, status, dto.NewErrorEnvelope(code, message, nil))
}
