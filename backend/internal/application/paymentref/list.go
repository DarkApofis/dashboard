package paymentref

import "context"

// ListReferences returns a filtered, paginated page of references.
type ListReferences struct {
	references Repository
	clock      Clock
}

// NewListReferences wires the use case with its ports.
func NewListReferences(references Repository, clock Clock) *ListReferences {
	return &ListReferences{references: references, clock: clock}
}

// Execute normalises the filter and delegates to the repository.
//
// The instant is read once and both passed down (so the status filter applies to
// the effective status) and returned (so the caller renders with the same instant
// the query used).
func (uc *ListReferences) Execute(ctx context.Context, filter Filter) (ListResult, error) {
	now := uc.clock.Now()
	filter.Normalize()

	page, err := uc.references.List(ctx, filter, now)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Page: page, ObservedAt: now}, nil
}
