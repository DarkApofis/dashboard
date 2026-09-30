package paymentref

import (
	"context"
	"errors"
	"time"

	"paymentlinks/internal/domain/business"
	"paymentlinks/internal/domain/errs"
	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
)

// ErrBusinessNotFound is returned when the body points at a business that does not
// exist. It is 422 rather than 404 because the route itself was found: the request
// is well formed but references something unusable.
var ErrBusinessNotFound = errs.Unprocessable("business_not_found", "business_id does not match an existing business")

// CreateInput is the use case input, already parsed but not yet validated: every
// business rule is checked by the domain entity, not here.
type CreateInput struct {
	BusinessID        int64
	ExternalReference string
	Amount            money.Money
	Description       string
	ExpiresAt         time.Time
}

// CreateReference creates a payment reference in the pending state.
type CreateReference struct {
	references Repository
	businesses BusinessFinder
	clock      Clock
}

// NewCreateReference wires the use case with its ports.
func NewCreateReference(references Repository, businesses BusinessFinder, clock Clock) *CreateReference {
	return &CreateReference{references: references, businesses: businesses, clock: clock}
}

// Execute validates and persists a new reference.
//
// The existence of the business is checked before building the entity so the
// client gets "business_not_found" instead of a foreign key error. That check is
// not race-proof on its own (the business could be deleted in between), which is
// why the repository also translates the foreign key violation; the check exists
// to produce a good error message, not to protect the invariant.
func (uc *CreateReference) Execute(ctx context.Context, in CreateInput) (Result, error) {
	now := uc.clock.Now()

	if in.BusinessID <= 0 {
		return Result{}, domain.ErrInvalidBusiness
	}

	exists, err := uc.businesses.ExistsByID(ctx, in.BusinessID)
	if err != nil {
		return Result{}, err
	}
	if !exists {
		return Result{}, ErrBusinessNotFound.WithDetail("business_id", formatID(in.BusinessID))
	}

	reference, err := domain.New(domain.NewParams{
		BusinessID:        in.BusinessID,
		ExternalReference: in.ExternalReference,
		Amount:            in.Amount,
		Description:       in.Description,
		ExpiresAt:         in.ExpiresAt,
		Now:               now,
	})
	if err != nil {
		return Result{}, err
	}

	if err := uc.references.Create(ctx, reference); err != nil {
		// A concurrent request could have deleted the business between the check
		// above and the insert; the repository reports it as business.ErrNotFound.
		if errors.Is(err, business.ErrNotFound) {
			return Result{}, ErrBusinessNotFound.WithDetail("business_id", formatID(in.BusinessID))
		}
		return Result{}, err
	}

	return Result{Reference: reference, ObservedAt: now}, nil
}
