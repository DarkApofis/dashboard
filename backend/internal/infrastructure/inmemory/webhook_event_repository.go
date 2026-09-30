package inmemory

import (
	"context"
	"sync"

	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
)

// WebhookEventRepository is an in-memory webhook event repository. The map key
// stands in for the unique index on event_id, so the idempotency behaviour the use
// case relies on is reproduced faithfully.
type WebhookEventRepository struct {
	mu         sync.Mutex
	byEventID  map[string]domain.WebhookEvent
	nextID     int64
	insertions int
}

var _ apppaymentref.WebhookEventRepository = (*WebhookEventRepository)(nil)

// NewWebhookEventRepository builds an empty repository.
func NewWebhookEventRepository() *WebhookEventRepository {
	return &WebhookEventRepository{byEventID: make(map[string]domain.WebhookEvent), nextID: 1}
}

// Insert records a delivery, or reports domain.ErrDuplicateEvent.
func (r *WebhookEventRepository) Insert(_ context.Context, event *domain.WebhookEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEventID[event.EventID]; exists {
		return domain.ErrDuplicateEvent.WithDetail("event_id", event.EventID)
	}

	event.ID = r.nextID
	r.nextID++
	r.byEventID[event.EventID] = *event
	r.insertions++
	return nil
}

// Finalize stamps the outcome and the resolved reference.
func (r *WebhookEventRepository) Finalize(_ context.Context, event *domain.WebhookEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.byEventID[event.EventID]
	if !ok {
		return domain.ErrNotFound
	}
	stored.Outcome = event.Outcome
	stored.PaymentReferenceID = event.PaymentReferenceID
	r.byEventID[event.EventID] = stored
	return nil
}

// GetByEventID returns a copy of the stored event.
func (r *WebhookEventRepository) GetByEventID(_ context.Context, eventID string) (*domain.WebhookEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.byEventID[eventID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &stored, nil
}

// Insertions counts the deliveries that were actually recorded, which is how a test
// asserts that a redelivery did not write a second row.
func (r *WebhookEventRepository) Insertions() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.insertions
}
