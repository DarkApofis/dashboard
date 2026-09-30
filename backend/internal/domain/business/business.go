// Package business holds the business entity: the party that issues payment
// references. It has no dependencies outside the domain.
package business

import (
	"strings"
	"time"

	"paymentlinks/internal/domain/errs"
)

// MaxNameLength bounds the stored name.
const MaxNameLength = 120

var (
	ErrNotFound    = errs.NotFound("business_not_found", "business not found")
	ErrInvalidName = errs.Unprocessable("invalid_business_name", "name must not be empty and must be at most 120 characters")
)

// Business is the owner of payment references.
type Business struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// New validates and builds a business. ID is assigned by the repository.
func New(name string, now time.Time) (*Business, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len([]rune(trimmed)) > MaxNameLength {
		return nil, ErrInvalidName
	}
	return &Business{Name: trimmed, CreatedAt: now.UTC()}, nil
}
