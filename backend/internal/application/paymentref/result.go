package paymentref

import (
	"time"

	domain "paymentlinks/internal/domain/paymentref"
)

// Result is what every single-reference use case returns.
//
// ObservedAt is the instant the use case read the clock, and it travels with the
// entity on purpose: the effective status of a reference depends on "now", so the
// caller that renders it must use the same instant the use case decided with.
// Otherwise a reference could be filtered as pending and rendered as expired in
// the same response.
type Result struct {
	Reference  *domain.PaymentReference
	ObservedAt time.Time
}

// EffectiveStatus is the status the caller should show.
func (r Result) EffectiveStatus() domain.Status {
	return r.Reference.EffectiveStatus(r.ObservedAt)
}

// ListResult is what the list use case returns, with the same reasoning.
type ListResult struct {
	Page       Page
	ObservedAt time.Time
}
