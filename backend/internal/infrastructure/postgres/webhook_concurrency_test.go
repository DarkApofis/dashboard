package postgres_test

// These tests wire the real webhook use case on top of the real PostgreSQL adapters
// and then race it against itself. They are the only place where the guarantees the
// design claims can actually be checked: a unique index that serialises deliveries,
// a row lock that serialises a payment against a cancellation, and a transaction
// boundary that makes either all of it or none of it visible.
//
// The database comes from TestMain, like the rest of the integration tests.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	app "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/business"
	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/infrastructure/clock"
	"paymentlinks/internal/infrastructure/postgres"
)

type webhookSuite struct {
	pool            *pgxpool.Pool
	references      *postgres.PaymentReferenceRepository
	registerPayment *app.RegisterPayment
	cancelReference *app.CancelReference
	reference       *domain.PaymentReference
}

func newWebhookSuite(t *testing.T) *webhookSuite {
	t.Helper()

	url := requireDatabase(t)

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: url, MaxConns: 8, ConnectRetries: 3}, nil)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE webhook_events, payment_references, businesses RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	references := postgres.NewPaymentReferenceRepository(pool)
	businesses := postgres.NewBusinessRepository(pool)
	events := postgres.NewWebhookEventRepository(pool)
	tx := postgres.NewTxManager(pool)
	fixed := clock.NewFixed(reference)

	owner, err := business.New("Acme Logistics", reference)
	if err != nil {
		t.Fatalf("build business: %v", err)
	}
	if err := businesses.Create(ctx, owner); err != nil {
		t.Fatalf("create business: %v", err)
	}

	paymentReference, err := domain.New(domain.NewParams{
		BusinessID:        owner.ID,
		ExternalReference: "INV-10023",
		Amount:            money.MustFromString("125000.50"),
		ExpiresAt:         reference.Add(24 * time.Hour),
		Now:               reference,
	})
	if err != nil {
		t.Fatalf("build reference: %v", err)
	}
	if err := references.Create(ctx, paymentReference); err != nil {
		t.Fatalf("create reference: %v", err)
	}

	return &webhookSuite{
		pool:            pool,
		references:      references,
		registerPayment: app.NewRegisterPayment(tx, references, events, fixed),
		cancelReference: app.NewCancelReference(tx, references, fixed),
		reference:       paymentReference,
	}
}

func (s *webhookSuite) input(eventID, transactionID string) app.RegisterPaymentInput {
	return app.RegisterPaymentInput{
		EventID:           eventID,
		ExternalReference: s.reference.ExternalReference,
		Amount:            s.reference.Amount,
		TransactionID:     transactionID,
		PaidAt:            reference.Add(-time.Minute),
		RawPayload:        []byte(`{"event_id":"` + eventID + `"}`),
	}
}

func (s *webhookSuite) countEvents(t *testing.T, eventID string) int {
	t.Helper()

	var count int
	err := s.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM webhook_events WHERE event_id = $1`, eventID).Scan(&count)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	return count
}

// The provider retries aggressively and its retries can overlap. Exactly one of them
// may apply the payment, and only one row may exist for the event.
func TestConcurrentRedeliveriesApplyOnce(t *testing.T) {
	s := newWebhookSuite(t)
	ctx := context.Background()

	const deliveries = 8
	results := make([]app.RegisterPaymentResult, deliveries)
	failures := make([]error, deliveries)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < deliveries; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release them all at once
			results[i], failures[i] = s.registerPayment.Execute(ctx, s.input("evt_race", "tx_883721"))
		}(i)
	}
	close(start)
	wg.Wait()

	var applied, duplicates int
	for i, err := range failures {
		if err != nil {
			t.Fatalf("delivery %d failed: %v", i, err)
		}
		switch {
		case results[i].Applied:
			applied++
		case results[i].Duplicate:
			duplicates++
		default:
			t.Errorf("delivery %d neither applied nor duplicate: %+v", i, results[i])
		}
	}

	if applied != 1 {
		t.Errorf("applied %d times, want exactly 1", applied)
	}
	if duplicates != deliveries-1 {
		t.Errorf("got %d duplicates, want %d", duplicates, deliveries-1)
	}
	if count := s.countEvents(t, "evt_race"); count != 1 {
		t.Errorf("stored %d rows for the event, want 1", count)
	}

	stored, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.StatusPaid {
		t.Errorf("status: got %q, want paid", stored.Status)
	}
	if stored.PaidTransactionID != "tx_883721" {
		t.Errorf("transaction_id: got %q", stored.PaidTransactionID)
	}
}

// Two different events for the same reference, arriving at once: the row lock decides
// the order, so one is applied and the other sees an already paid reference.
func TestConcurrentDistinctEventsOnTheSameReference(t *testing.T) {
	s := newWebhookSuite(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	outcomes := make([]domain.WebhookOutcome, 2)
	failures := make([]error, 2)

	start := make(chan struct{})
	for i, input := range []app.RegisterPaymentInput{
		s.input("evt_a", "tx_a"),
		s.input("evt_b", "tx_b"),
	} {
		wg.Add(1)
		go func(i int, in app.RegisterPaymentInput) {
			defer wg.Done()
			<-start
			result, err := s.registerPayment.Execute(ctx, in)
			outcomes[i], failures[i] = result.Outcome, err
		}(i, input)
	}
	close(start)
	wg.Wait()

	for i, err := range failures {
		if err != nil {
			t.Fatalf("delivery %d failed: %v", i, err)
		}
	}

	var appliedCount, conflictCount int
	for _, outcome := range outcomes {
		switch outcome {
		case domain.OutcomeApplied:
			appliedCount++
		case domain.OutcomeConflictOtherTransaction:
			conflictCount++
		default:
			t.Errorf("unexpected outcome %q", outcome)
		}
	}
	if appliedCount != 1 || conflictCount != 1 {
		t.Errorf("got %d applied and %d conflicts, want 1 and 1", appliedCount, conflictCount)
	}

	// Both events are recorded, because both were genuinely delivered.
	if total := s.countEvents(t, "evt_a") + s.countEvents(t, "evt_b"); total != 2 {
		t.Errorf("stored %d events, want 2", total)
	}

	stored, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !stored.RequiresReview {
		t.Error("two transactions against one reference must be flagged for review")
	}
}

// The race the FOR UPDATE in both use cases exists for: a payment and a cancellation
// competing for the same reference. Whoever commits first wins, and the loser is told
// why. What must never happen is a row that looks both paid and cancelled.
func TestPaymentRacesCancellation(t *testing.T) {
	s := newWebhookSuite(t)
	ctx := context.Background()

	var (
		wg         sync.WaitGroup
		webhookOut domain.WebhookOutcome
		webhookErr error
		cancelErr  error
		start      = make(chan struct{})
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		result, err := s.registerPayment.Execute(ctx, s.input("evt_race", "tx_883721"))
		webhookOut, webhookErr = result.Outcome, err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, cancelErr = s.cancelReference.Execute(ctx, s.reference.ID)
	}()
	close(start)
	wg.Wait()

	if webhookErr != nil {
		t.Fatalf("the webhook must never fail on a business conflict: %v", webhookErr)
	}

	stored, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	switch stored.Status {
	case domain.StatusPaid:
		// The payment committed first, so the cancellation had to be refused.
		if !errors.Is(cancelErr, domain.ErrCannotCancelPaid) {
			t.Errorf("cancel error: got %v, want ErrCannotCancelPaid", cancelErr)
		}
		if webhookOut != domain.OutcomeApplied {
			t.Errorf("webhook outcome: got %q, want applied", webhookOut)
		}
		if stored.CancelledAt != nil {
			t.Error("a paid reference must not carry a cancellation timestamp")
		}
	case domain.StatusCancelled:
		// The cancellation committed first, so the payment had to be recorded as a
		// conflict rather than applied.
		if cancelErr != nil {
			t.Errorf("cancel error: got %v, want nil", cancelErr)
		}
		if webhookOut != domain.OutcomeConflictCancelled {
			t.Errorf("webhook outcome: got %q, want conflict_cancelled", webhookOut)
		}
		if stored.PaidAt != nil || stored.PaidTransactionID != "" {
			t.Errorf("a cancelled reference must carry no payment: paid_at=%v tx=%q",
				stored.PaidAt, stored.PaidTransactionID)
		}
		if !stored.RequiresReview {
			t.Error("a payment for a cancelled reference must be flagged for review")
		}
	default:
		t.Fatalf("the reference ended in %q, which means neither operation completed", stored.Status)
	}

	// Either way the delivery is on record.
	if count := s.countEvents(t, "evt_race"); count != 1 {
		t.Errorf("stored %d rows for the event, want 1", count)
	}
}

// The sequential version of the idempotency guarantee, against the real unique index:
// the same event delivered twice must produce exactly one state change.
func TestSameEventIDProducesOneStateChange(t *testing.T) {
	s := newWebhookSuite(t)
	ctx := context.Background()

	first, err := s.registerPayment.Execute(ctx, s.input("evt_983829", "tx_883721"))
	if err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if !first.Applied {
		t.Fatalf("the first delivery must apply the payment, got %+v", first)
	}

	afterFirst, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	second, err := s.registerPayment.Execute(ctx, s.input("evt_983829", "tx_883721"))
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if second.Applied {
		t.Error("the redelivery must not apply anything")
	}
	if !second.Duplicate {
		t.Error("the redelivery must be reported as a duplicate")
	}
	if second.Outcome != first.Outcome {
		t.Errorf("outcome: got %q, want the original %q", second.Outcome, first.Outcome)
	}

	afterSecond, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !afterSecond.UpdatedAt.Equal(afterFirst.UpdatedAt) {
		t.Errorf("updated_at moved from %v to %v: the row was written twice",
			afterFirst.UpdatedAt, afterSecond.UpdatedAt)
	}
	if count := s.countEvents(t, "evt_983829"); count != 1 {
		t.Errorf("stored %d rows for the event, want 1", count)
	}
}

// A notified amount that does not match leaves the reference exactly as it was, and
// the delivery on record for whoever reconciles it.
func TestAmountMismatchKeepsReferencePending(t *testing.T) {
	s := newWebhookSuite(t)
	ctx := context.Background()

	mismatched := s.input("evt_mismatch", "tx_883721")
	mismatched.Amount = money.MustFromString("125000.49") // one cent short

	result, err := s.registerPayment.Execute(ctx, mismatched)
	if err != nil {
		t.Fatalf("Execute: unexpected error %v (a business rejection is not an error)", err)
	}
	if result.Outcome != domain.OutcomeAmountMismatch {
		t.Errorf("outcome: got %q, want amount_mismatch", result.Outcome)
	}
	if result.Applied {
		t.Error("a mismatched amount must not be applied")
	}

	stored, err := s.references.GetByID(ctx, s.reference.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.StatusPending {
		t.Errorf("status: got %q, want pending", stored.Status)
	}
	if stored.PaidAt != nil || stored.PaidTransactionID != "" {
		t.Errorf("no payment data may be stored: paid_at=%v tx=%q", stored.PaidAt, stored.PaidTransactionID)
	}
	if !stored.RequiresReview || stored.ReviewReason != domain.ReviewReasonAmountMismatch {
		t.Errorf("got requires_review=%v reason=%q", stored.RequiresReview, stored.ReviewReason)
	}
	// The amount itself is untouched, so the discrepancy stays visible.
	if stored.Amount.String() != "125000.50" {
		t.Errorf("amount: got %q, want the original 125000.50", stored.Amount.String())
	}

	var storedOutcome, storedAmount string
	err = s.pool.QueryRow(ctx,
		`SELECT outcome, amount::text FROM webhook_events WHERE event_id = $1`, "evt_mismatch").
		Scan(&storedOutcome, &storedAmount)
	if err != nil {
		t.Fatalf("read the recorded event: %v", err)
	}
	if storedOutcome != string(domain.OutcomeAmountMismatch) {
		t.Errorf("recorded outcome: got %q", storedOutcome)
	}
	if storedAmount != "125000.49" {
		t.Errorf("the notified amount must be recorded as received: got %q", storedAmount)
	}
}
