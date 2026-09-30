package paymentref

import (
	domain "paymentlinks/internal/domain/paymentref"
)

// Pagination bounds. The spec does not mention pagination, but an unbounded list
// endpoint does not survive contact with real data, so the list is always capped.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Filter describes an optional-by-field query over payment references. Pointer
// fields distinguish "not filtering" from "filtering by the zero value".
type Filter struct {
	BusinessID        *int64
	Status            *domain.Status
	ExternalReference *string
	Limit             int
	Offset            int
}

// Normalize clamps pagination into the supported range.
func (f *Filter) Normalize() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// Page is one slice of results plus the total number of matches, which the
// dashboard needs to render pagination.
type Page struct {
	Items  []*domain.PaymentReference
	Total  int
	Limit  int
	Offset int
}
