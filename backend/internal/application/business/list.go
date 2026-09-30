package business

import (
	"context"

	domain "paymentlinks/internal/domain/business"
)

// ListBusinesses returns every business, for the selector in the dashboard.
type ListBusinesses struct {
	businesses Repository
}

// NewListBusinesses wires the use case with its port.
func NewListBusinesses(businesses Repository) *ListBusinesses {
	return &ListBusinesses{businesses: businesses}
}

// Execute returns the businesses ordered by the repository (alphabetically). The
// list is not paginated: there is one row per client company, so the whole table
// fits in a response and the frontend can fill a select in one call.
func (uc *ListBusinesses) Execute(ctx context.Context) ([]*domain.Business, error) {
	return uc.businesses.List(ctx)
}
