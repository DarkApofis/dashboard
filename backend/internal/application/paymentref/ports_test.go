package paymentref_test

// Test doubles that the happy-path fakes cannot replace.
//
// internal/infrastructure/inmemory behaves like a working database, which is what
// most use case tests need. These two do the opposite:
//
//   - spyReferences fails on demand, so error propagation can be checked. A use case
//     that swallows a repository failure would report success on a write that never
//     happened, which is the worst bug this codebase could have.
//   - recordingTxManager records the transaction boundary, so the claim that the
//     webhook and the cancellation run inside one transaction is asserted rather
//     than assumed.

import (
	"context"
	"sync"
	"testing"
	"time"

	app "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/infrastructure/inmemory"
)

// txMarker is put in the context by recordingTxManager and looked for by the spies.
type txMarker struct{}

func inTransaction(ctx context.Context) bool {
	marked, _ := ctx.Value(txMarker{}).(bool)
	return marked
}

// recordingTxManager counts the transactions opened and marks their context.
type recordingTxManager struct {
	mu    sync.Mutex
	calls int
}

var _ app.TxManager = (*recordingTxManager)(nil)

func (m *recordingTxManager) WithTx(ctx context.Context, fn func(context.Context) error) error {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()

	if inTransaction(ctx) {
		return fn(ctx) // a nested call joins the outer transaction
	}
	return fn(context.WithValue(ctx, txMarker{}, true))
}

func (m *recordingTxManager) transactions() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// spyReferences decorates the in-memory repository with failure injection and a
// record of which methods were called inside a transaction.
type spyReferences struct {
	*inmemory.PaymentReferenceRepository

	mu       sync.Mutex
	failOn   map[string]error
	calledIn map[string]bool
}

var _ app.Repository = (*spyReferences)(nil)

func newSpyReferences() *spyReferences {
	return &spyReferences{
		PaymentReferenceRepository: inmemory.NewPaymentReferenceRepository(),
		failOn:                     make(map[string]error),
		calledIn:                   make(map[string]bool),
	}
}

// fail makes the named method return err from then on.
func (s *spyReferences) fail(method string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOn[method] = err
}

// sawTransaction reports whether the named method ran inside a transaction.
func (s *spyReferences) sawTransaction(method string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calledIn[method]
}

func (s *spyReferences) record(ctx context.Context, method string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inTransaction(ctx) {
		s.calledIn[method] = true
	}
	return s.failOn[method]
}

func (s *spyReferences) Create(ctx context.Context, ref *domain.PaymentReference) error {
	if err := s.record(ctx, "Create"); err != nil {
		return err
	}
	return s.PaymentReferenceRepository.Create(ctx, ref)
}

func (s *spyReferences) GetByID(ctx context.Context, id int64) (*domain.PaymentReference, error) {
	if err := s.record(ctx, "GetByID"); err != nil {
		return nil, err
	}
	return s.PaymentReferenceRepository.GetByID(ctx, id)
}

func (s *spyReferences) LockByID(ctx context.Context, id int64) (*domain.PaymentReference, error) {
	if err := s.record(ctx, "LockByID"); err != nil {
		return nil, err
	}
	return s.PaymentReferenceRepository.LockByID(ctx, id)
}

func (s *spyReferences) LockByExternalReference(ctx context.Context, externalReference string) (*domain.PaymentReference, error) {
	if err := s.record(ctx, "LockByExternalReference"); err != nil {
		return nil, err
	}
	return s.PaymentReferenceRepository.LockByExternalReference(ctx, externalReference)
}

func (s *spyReferences) List(ctx context.Context, filter app.Filter, now time.Time) (app.Page, error) {
	if err := s.record(ctx, "List"); err != nil {
		return app.Page{}, err
	}
	return s.PaymentReferenceRepository.List(ctx, filter, now)
}

func (s *spyReferences) UpdateWithStatusGuard(ctx context.Context, ref *domain.PaymentReference, expected domain.Status) error {
	if err := s.record(ctx, "UpdateWithStatusGuard"); err != nil {
		return err
	}
	return s.PaymentReferenceRepository.UpdateWithStatusGuard(ctx, ref, expected)
}

// spyEvents is the same idea for the webhook event repository.
type spyEvents struct {
	*inmemory.WebhookEventRepository

	mu       sync.Mutex
	failOn   map[string]error
	calledIn map[string]bool
}

var _ app.WebhookEventRepository = (*spyEvents)(nil)

func newSpyEvents() *spyEvents {
	return &spyEvents{
		WebhookEventRepository: inmemory.NewWebhookEventRepository(),
		failOn:                 make(map[string]error),
		calledIn:               make(map[string]bool),
	}
}

func (s *spyEvents) fail(method string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOn[method] = err
}

func (s *spyEvents) sawTransaction(method string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calledIn[method]
}

func (s *spyEvents) record(ctx context.Context, method string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inTransaction(ctx) {
		s.calledIn[method] = true
	}
	return s.failOn[method]
}

func (s *spyEvents) Insert(ctx context.Context, event *domain.WebhookEvent) error {
	if err := s.record(ctx, "Insert"); err != nil {
		return err
	}
	return s.WebhookEventRepository.Insert(ctx, event)
}

func (s *spyEvents) Finalize(ctx context.Context, event *domain.WebhookEvent) error {
	if err := s.record(ctx, "Finalize"); err != nil {
		return err
	}
	return s.WebhookEventRepository.Finalize(ctx, event)
}

func (s *spyEvents) GetByEventID(ctx context.Context, eventID string) (*domain.WebhookEvent, error) {
	if err := s.record(ctx, "GetByEventID"); err != nil {
		return nil, err
	}
	return s.WebhookEventRepository.GetByEventID(ctx, eventID)
}

// spyBusinesses fails the existence check on demand.
type spyBusinesses struct {
	*inmemory.BusinessRepository
	err error
}

var _ app.BusinessFinder = (*spyBusinesses)(nil)

func (s *spyBusinesses) ExistsByID(ctx context.Context, id int64) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.BusinessRepository.ExistsByID(ctx, id)
}

// assertNoError is used where a helper failure would otherwise be silent.
func assertNoError(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}
