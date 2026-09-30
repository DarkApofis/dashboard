// Package paymentref holds the payment reference entity, its state machine and
// its domain errors. It imports nothing from HTTP, SQL or any framework: every
// rule in here is expressible and testable with plain values.
package paymentref

import (
	"strings"
	"time"
	"unicode"

	"paymentlinks/internal/domain/money"
)

const (
	MaxExternalReferenceLength = 64
	MaxDescriptionLength       = 500
)

// ReviewReasonPaidAfterExpiration marks a reference paid after its expiration
// date. The payment is accepted (the money did arrive) but flagged so a human
// decides whether to honour or refund it.
const ReviewReasonPaidAfterExpiration = "paid_after_expiration"

// PaymentReference is the amount a business asks a customer to pay, identified
// towards the financial provider by its external reference.
type PaymentReference struct {
	ID                int64
	BusinessID        int64
	ExternalReference string
	Amount            money.Money
	Description       string
	Status            Status
	ExpiresAt         time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Audit fields. Not required by the spec, but without them a status cannot
	// be explained after the fact.
	PaidAt            *time.Time
	CancelledAt       *time.Time
	PaidTransactionID string
	RequiresReview    bool
	ReviewReason      string
}

// NewParams is the input of New. Now is injected rather than read from a clock so
// that every time-dependent rule is deterministic in tests.
type NewParams struct {
	BusinessID        int64
	ExternalReference string
	Amount            money.Money
	Description       string
	ExpiresAt         time.Time
	Now               time.Time
}

// New validates the invariants of a brand new reference and returns it pending.
// The ID is assigned by the repository.
func New(p NewParams) (*PaymentReference, error) {
	if p.BusinessID <= 0 {
		return nil, ErrInvalidBusiness
	}

	externalReference := strings.TrimSpace(p.ExternalReference)
	if !isValidExternalReference(externalReference) {
		return nil, ErrInvalidExternalReference.WithDetail("external_reference", p.ExternalReference)
	}

	if !p.Amount.IsPositive() {
		return nil, ErrAmountNotPositive
	}

	description := strings.TrimSpace(p.Description)
	if len([]rune(description)) > MaxDescriptionLength {
		return nil, ErrDescriptionTooLong
	}

	if p.ExpiresAt.IsZero() || !p.ExpiresAt.After(p.Now) {
		return nil, ErrExpirationNotInFuture
	}

	now := p.Now.UTC()
	return &PaymentReference{
		BusinessID:        p.BusinessID,
		ExternalReference: externalReference,
		Amount:            p.Amount,
		Description:       description,
		Status:            StatusPending,
		ExpiresAt:         p.ExpiresAt.UTC(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// IsExpiredAt reports whether a still pending reference is past its expiration
// date. A stored status of pending is not trusted on its own: the periodic job
// that materialises expirations may not have run yet.
func (r *PaymentReference) IsExpiredAt(now time.Time) bool {
	return r.Status == StatusPending && !r.ExpiresAt.After(now)
}

// EffectiveStatus is the status callers should see and act on.
func (r *PaymentReference) EffectiveStatus(now time.Time) Status {
	if r.IsExpiredAt(now) {
		return StatusExpired
	}
	return r.Status
}

// Cancel moves a pending reference to cancelled.
//
// Cancelling an already cancelled reference is a no-op that reports success:
// cancel expresses a desired end state, so a client retry must not fail. A paid
// reference can never be cancelled, and an expired one has nothing to cancel.
func (r *PaymentReference) Cancel(now time.Time) error {
	switch r.EffectiveStatus(now) {
	case StatusCancelled:
		return nil
	case StatusPaid:
		return ErrCannotCancelPaid
	case StatusExpired:
		return ErrCannotCancelExpired
	}

	if err := r.transitionTo(StatusCancelled); err != nil {
		return err
	}
	cancelledAt := now.UTC()
	r.CancelledAt = &cancelledAt
	r.touch(now)
	return nil
}

// MarkAsPaid applies a payment confirmed by the financial provider.
//
// An expired reference is still marked paid and flagged for review: rejecting it
// would leave the business having received money with no record of it. A
// cancelled reference is not, because cancellation is an explicit business
// decision; the caller records the conflict instead of overriding it silently.
func (r *PaymentReference) MarkAsPaid(transactionID string, paidAt, now time.Time) error {
	if strings.TrimSpace(transactionID) == "" {
		return ErrInvalidTransactionID
	}

	switch r.EffectiveStatus(now) {
	case StatusPaid:
		return ErrAlreadyPaid
	case StatusCancelled:
		return ErrCannotPayCancelled
	}

	paidAfterExpiration := r.IsExpiredAt(now) || r.Status == StatusExpired
	if err := r.transitionTo(StatusPaid); err != nil {
		return err
	}

	paidAtUTC := paidAt.UTC()
	r.PaidAt = &paidAtUTC
	r.PaidTransactionID = strings.TrimSpace(transactionID)
	if paidAfterExpiration {
		r.FlagForReview(ReviewReasonPaidAfterExpiration)
	}
	r.touch(now)
	return nil
}

// Expire materialises the expiration of a pending reference whose date passed.
func (r *PaymentReference) Expire(now time.Time) error {
	if r.Status != StatusPending {
		return ErrInvalidTransition.WithMessage("cannot expire a reference in status %s", r.Status)
	}
	if r.ExpiresAt.After(now) {
		return ErrNotYetExpired
	}
	if err := r.transitionTo(StatusExpired); err != nil {
		return err
	}
	r.touch(now)
	return nil
}

// FlagForReview marks the reference as needing human reconciliation.
func (r *PaymentReference) FlagForReview(reason string) {
	r.RequiresReview = true
	r.ReviewReason = reason
}

func (r *PaymentReference) transitionTo(target Status) error {
	if !r.Status.CanTransitionTo(target) {
		return ErrInvalidTransition.WithMessage("cannot transition from %s to %s", r.Status, target)
	}
	r.Status = target
	return nil
}

func (r *PaymentReference) touch(now time.Time) { r.UpdatedAt = now.UTC() }

func isValidExternalReference(value string) bool {
	if value == "" || len(value) > MaxExternalReferenceLength {
		return false
	}
	for _, char := range value {
		if unicode.IsSpace(char) || !unicode.IsPrint(char) {
			return false
		}
	}
	return true
}
