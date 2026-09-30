// Package money models monetary amounts exactly.
//
// Amounts are base-10 decimals with at most two fractional digits, mirroring the
// NUMERIC(14,2) columns they are stored in. float64 is never used: 125000.50 is
// not representable in binary, so a float would break both equality checks (the
// webhook compares the notified amount against the expected one) and sums.
package money

import (
	"strings"

	"github.com/shopspring/decimal"

	"paymentlinks/internal/domain/errs"
)

// Scale is the number of fractional digits an amount may carry.
const Scale = 2

// maxAmount matches the precision of the NUMERIC(14,2) column.
var maxAmount = decimal.RequireFromString("999999999999.99")

var (
	ErrInvalidFormat = errs.Validation("invalid_amount_format", "amount must be a decimal number")
	ErrNegative      = errs.Unprocessable("negative_amount", "amount cannot be negative")
	ErrScale         = errs.Unprocessable("amount_scale_exceeded", "amount cannot have more than 2 decimal places")
	ErrTooLarge      = errs.Unprocessable("amount_too_large", "amount exceeds the maximum supported value")
)

// Money is an immutable exact amount. Its zero value is a valid zero amount.
type Money struct {
	value decimal.Decimal
}

// Zero is the zero amount.
func Zero() Money { return Money{} }

// FromDecimal validates an already parsed decimal.
func FromDecimal(d decimal.Decimal) (Money, error) {
	if d.Sign() < 0 {
		return Money{}, ErrNegative
	}
	if !d.Truncate(Scale).Equal(d) {
		return Money{}, ErrScale
	}
	if d.GreaterThan(maxAmount) {
		return Money{}, ErrTooLarge
	}
	// Normalise the exponent so that "10", "10.0" and "10.00" compare and
	// serialise identically.
	return Money{value: d.Round(Scale)}, nil
}

// FromString parses a decimal string such as "125000.50".
func FromString(raw string) (Money, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Money{}, ErrInvalidFormat
	}
	d, err := decimal.NewFromString(trimmed)
	if err != nil {
		return Money{}, ErrInvalidFormat.Wrap(err)
	}
	return FromDecimal(d)
}

// MustFromString is FromString for constants and tests; it panics on bad input.
func MustFromString(raw string) Money {
	m, err := FromString(raw)
	if err != nil {
		panic("money: invalid literal " + raw + ": " + err.Error())
	}
	return m
}

// Decimal exposes the underlying value for callers that need arithmetic.
func (m Money) Decimal() decimal.Decimal { return m.value }

// String renders the amount with exactly two decimals ("125000.50"). This is
// also the representation sent to and read back from PostgreSQL.
func (m Money) String() string { return m.value.StringFixed(Scale) }

// Equal is exact equality; use it instead of ==, which compares internals.
func (m Money) Equal(other Money) bool { return m.value.Equal(other.value) }

// IsPositive reports whether the amount is strictly greater than zero.
func (m Money) IsPositive() bool { return m.value.Sign() > 0 }

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.value.Sign() == 0 }
