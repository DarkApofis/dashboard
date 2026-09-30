// Package dto holds the wire shapes of the HTTP API. They are separate from the
// domain entities on purpose: the API contract and the internal model must be
// free to evolve at different speeds.
package dto

// ErrorEnvelope is the single error shape of the whole API:
//
//	{ "error": { "code": "...", "message": "..." } }
//
// The request id is not duplicated in the body; it travels in the X-Request-ID
// response header on every response, error or not.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the payload of an error response. Code is stable and meant to be
// switched on by clients; Message is for humans. Details only appears for
// field-level validation failures.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// NewErrorEnvelope builds the envelope.
func NewErrorEnvelope(code, message string, details map[string]string) ErrorEnvelope {
	return ErrorEnvelope{Error: ErrorBody{Code: code, Message: message, Details: details}}
}
