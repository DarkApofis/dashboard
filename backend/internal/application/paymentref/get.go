package paymentref

import (
	"context"
	"strconv"
)

// GetReference reads a single payment reference.
type GetReference struct {
	references Repository
	clock      Clock
}

// NewGetReference wires the use case with its ports.
func NewGetReference(references Repository, clock Clock) *GetReference {
	return &GetReference{references: references, clock: clock}
}

// Execute returns the reference, or domain.ErrNotFound. The stored status is
// returned untouched; the effective status is derived by the caller from
// Result.ObservedAt, so a read never writes.
func (uc *GetReference) Execute(ctx context.Context, id int64) (Result, error) {
	now := uc.clock.Now()

	reference, err := uc.references.GetByID(ctx, id)
	if err != nil {
		return Result{}, err
	}
	return Result{Reference: reference, ObservedAt: now}, nil
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }
