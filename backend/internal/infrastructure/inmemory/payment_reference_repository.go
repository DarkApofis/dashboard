// Package inmemory implements the application ports with in-memory maps.
//
// It is the payoff of declaring the ports as interfaces: the use cases and the
// handlers can be tested end to end, including status codes, without a database
// and without mocking frameworks. It is a test double, not a production adapter:
// the transaction manager does not roll back, so tests written against it assert
// behaviour, never atomicity.
package inmemory

import (
	"context"
	"sort"
	"sync"
	"time"

	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
)

// PaymentReferenceRepository is an in-memory payment reference repository.
type PaymentReferenceRepository struct {
	mu     sync.Mutex
	byID   map[int64]domain.PaymentReference
	nextID int64
}

var _ apppaymentref.Repository = (*PaymentReferenceRepository)(nil)

// NewPaymentReferenceRepository builds an empty repository.
func NewPaymentReferenceRepository() *PaymentReferenceRepository {
	return &PaymentReferenceRepository{byID: make(map[int64]domain.PaymentReference), nextID: 1}
}

// Create stores a copy and assigns the next id, rejecting a duplicate external
// reference the same way the unique index does.
func (r *PaymentReferenceRepository) Create(_ context.Context, reference *domain.PaymentReference) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, stored := range r.byID {
		if stored.ExternalReference == reference.ExternalReference {
			return domain.ErrDuplicateReference.WithDetail("external_reference", reference.ExternalReference)
		}
	}

	reference.ID = r.nextID
	r.nextID++
	r.byID[reference.ID] = *reference
	return nil
}

// GetByID returns a copy, so a caller mutating the entity cannot alter the store
// without going through Update, exactly like the SQL repository.
func (r *PaymentReferenceRepository) GetByID(_ context.Context, id int64) (*domain.PaymentReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &stored, nil
}

// GetByExternalReference returns a copy keyed by external reference.
func (r *PaymentReferenceRepository) GetByExternalReference(_ context.Context, externalReference string) (*domain.PaymentReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, stored := range r.byID {
		if stored.ExternalReference == externalReference {
			copied := stored
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

// LockByID behaves as GetByID: there is no row locking to emulate here.
func (r *PaymentReferenceRepository) LockByID(ctx context.Context, id int64) (*domain.PaymentReference, error) {
	return r.GetByID(ctx, id)
}

// LockByExternalReference behaves as GetByExternalReference.
func (r *PaymentReferenceRepository) LockByExternalReference(ctx context.Context, externalReference string) (*domain.PaymentReference, error) {
	return r.GetByExternalReference(ctx, externalReference)
}

// List applies the same semantics as the SQL implementation: filters on the
// effective status, newest first, total independent of pagination.
func (r *PaymentReferenceRepository) List(_ context.Context, filter apppaymentref.Filter, now time.Time) (apppaymentref.Page, error) {
	filter.Normalize()

	r.mu.Lock()
	defer r.mu.Unlock()

	matches := make([]*domain.PaymentReference, 0, len(r.byID))
	for _, stored := range r.byID {
		candidate := stored
		if filter.BusinessID != nil && candidate.BusinessID != *filter.BusinessID {
			continue
		}
		if filter.ExternalReference != nil && candidate.ExternalReference != *filter.ExternalReference {
			continue
		}
		if filter.Status != nil && candidate.EffectiveStatus(now) != *filter.Status {
			continue
		}
		matches = append(matches, &candidate)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].ID > matches[j].ID
		}
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})

	total := len(matches)
	start := min(filter.Offset, total)
	end := min(start+filter.Limit, total)

	return apppaymentref.Page{
		Items:  matches[start:end],
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

// UpdateWithStatusGuard emulates the conditional UPDATE, including the failure
// mode: if the stored status is no longer the expected one, the write is refused.
func (r *PaymentReferenceRepository) UpdateWithStatusGuard(_ context.Context, reference *domain.PaymentReference, expected domain.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.byID[reference.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if stored.Status != expected {
		return domain.ErrConcurrentUpdate.WithDetail("expected_status", string(expected))
	}

	r.byID[reference.ID] = *reference
	return nil
}

// ExpireDue moves overdue pending references to expired, up to limit.
func (r *PaymentReferenceRepository) ExpireDue(_ context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var expired int64
	for id, stored := range r.byID {
		if expired >= int64(limit) {
			break
		}
		candidate := stored
		if err := candidate.Expire(now); err != nil {
			continue
		}
		r.byID[id] = candidate
		expired++
	}
	return expired, nil
}
