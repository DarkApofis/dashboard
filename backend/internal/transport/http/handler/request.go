package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"paymentlinks/internal/domain/errs"
)

// maxBodyBytes caps the request body. The payloads of this API are a few hundred
// bytes; anything larger is a mistake or an attack, and rejecting it early keeps a
// single request from consuming memory.
const maxBodyBytes = 64 << 10 // 64 KiB

// Transport-level errors: the request could not even be interpreted, so they never
// reach the domain and always map to 400.
var (
	errEmptyBody    = errs.Validation("empty_body", "a JSON body is required")
	errInvalidBody  = errs.Validation("invalid_json_body", "the request body is not valid JSON")
	errUnknownField = errs.Validation("unknown_field", "the request body contains a field this endpoint does not accept")
	errInvalidID    = errs.Validation("invalid_id", "the id in the path must be a positive integer")
	errInvalidQuery = errs.Validation("invalid_query_parameter", "a query parameter has an invalid value")
)

// decodeJSON reads the body into dst.
//
// Unknown fields are rejected so a client typo ("amount_cents" instead of "amount")
// surfaces as an error instead of being silently ignored, which is the kind of bug
// that only shows up once money is involved.
//
// A domain error raised while decoding (an amount with three decimals, for example)
// is returned as is, so it keeps its own status code instead of collapsing into a
// generic 400.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errEmptyBody
	}

	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if domainErr, ok := errs.From(err); ok {
			return domainErr
		}
		switch {
		case errors.Is(err, io.EOF):
			return errEmptyBody
		case strings.Contains(err.Error(), "unknown field"):
			return errUnknownField.WithMessage("%s", err.Error())
		default:
			return errInvalidBody.WithDetail("reason", err.Error())
		}
	}

	// A single JSON document per request: trailing content means the client built
	// the payload wrong.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errInvalidBody.WithDetail("reason", "the body must contain a single JSON object")
	}
	return nil
}

// readJSON reads the body, returns it verbatim and decodes it into dst.
//
// Unlike decodeJSON it tolerates unknown fields, because it is used for payloads
// whose contract belongs to a third party: rejecting a delivery over a field the
// provider added would turn their release into our outage. The raw bytes are
// returned so they can be stored for auditing.
func readJSON(r *http.Request, dst any) ([]byte, error) {
	if r.Body == nil {
		return nil, errEmptyBody
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return nil, errInvalidBody.WithDetail("reason", "the body could not be read")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errEmptyBody
	}

	if err := json.Unmarshal(raw, dst); err != nil {
		// A domain error raised while decoding (an amount with three decimals, for
		// instance) keeps its own status code.
		if domainErr, ok := errs.From(err); ok {
			return nil, domainErr
		}
		return nil, errInvalidBody.WithDetail("reason", err.Error())
	}
	return raw, nil
}

// pathID parses a positive integer path parameter.
func pathID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidID.WithDetail("id", raw)
	}
	return id, nil
}

// queryInt64 reads an optional positive integer query parameter. A missing or empty
// parameter means "no filter", not zero.
func queryInt64(values map[string][]string, keys ...string) (*int64, error) {
	raw, ok := firstNonEmpty(values, keys...)
	if !ok {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return nil, errInvalidQuery.WithDetail(keys[0], raw)
	}
	return &parsed, nil
}

// queryInt reads an optional non-negative integer query parameter.
func queryInt(values map[string][]string, key string, fallback int) (int, error) {
	raw, ok := firstNonEmpty(values, key)
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0, errInvalidQuery.WithDetail(key, raw)
	}
	return parsed, nil
}

// queryString reads an optional string query parameter.
func queryString(values map[string][]string, keys ...string) *string {
	raw, ok := firstNonEmpty(values, keys...)
	if !ok {
		return nil
	}
	return &raw
}

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }

func firstNonEmpty(values map[string][]string, keys ...string) (string, bool) {
	for _, key := range keys {
		for _, candidate := range values[key] {
			if trimmed := strings.TrimSpace(candidate); trimmed != "" {
				return trimmed, true
			}
		}
	}
	return "", false
}
