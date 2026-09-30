package dto

import (
	"bytes"
	"strings"

	"paymentlinks/internal/domain/money"
)

// Amount is a monetary amount on the wire.
//
// It exists because encoding/json decodes a JSON number into float64, and 125000.50
// has no exact float64 representation. Parsing the raw bytes instead means the
// value never passes through a float: the amount that reaches the domain is exactly
// the one the client sent, which is what makes the webhook amount comparison
// trustworthy.
//
// Both 125000.50 and "125000.50" are accepted on input; output is always a JSON
// number with two decimals, matching the contract in the spec.
type Amount struct {
	Value money.Money
}

// NewAmount wraps a domain amount for serialisation.
func NewAmount(value money.Money) Amount { return Amount{Value: value} }

// UnmarshalJSON parses the amount without going through float64.
func (a *Amount) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(bytes.TrimSpace(data)))
	if raw == "null" {
		return nil
	}
	// A quoted amount is accepted so a JavaScript client can avoid its own float
	// problems by sending the value as a string.
	raw = strings.Trim(raw, `"`)

	parsed, err := money.FromString(raw)
	if err != nil {
		return err
	}
	a.Value = parsed
	return nil
}

// MarshalJSON writes the amount as a JSON number with exactly two decimals.
func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.Value.String()), nil
}
