package paymentref_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/infrastructure/clock"
	"paymentlinks/internal/infrastructure/inmemory"
)

// errDatabase stands for anything unexpected coming out of the infrastructure: a
// dropped connection, a statement timeout, a disk full.
var errDatabase = errors.New("connection reset by peer")

// spyFixture is newFixture with the failure-injecting doubles in place of the plain
// in-memory ones.
type spyFixture struct {
	references *spyReferences
	events     *spyEvents
	businesses *spyBusinesses
	tx         *recordingTxManager
	clock      *clock.Fixed
	businessID int64

	create          *app.CreateReference
	get             *app.GetReference
	list            *app.ListReferences
	cancel          *app.CancelReference
	registerPayment *app.RegisterPayment
}

func newSpyFixture(t *testing.T) *spyFixture {
	t.Helper()

	references := newSpyReferences()
	businesses := &spyBusinesses{BusinessRepository: inmemory.NewBusinessRepository()}
	events := newSpyEvents()
	tx := &recordingTxManager{}
	fixed := clock.NewFixed(reference)
	ids := businesses.Seed("Acme Logistics")

	return &spyFixture{
		references:      references,
		events:          events,
		businesses:      businesses,
		tx:              tx,
		clock:           fixed,
		businessID:      ids[0],
		create:          app.NewCreateReference(references, businesses, fixed),
		get:             app.NewGetReference(references, fixed),
		list:            app.NewListReferences(references, fixed),
		cancel:          app.NewCancelReference(tx, references, fixed),
		registerPayment: app.NewRegisterPayment(tx, references, events, fixed),
	}
}

func (f *spyFixture) seed(t *testing.T, externalReference string) *domain.PaymentReference {
	t.Helper()

	result, err := f.create.Execute(context.Background(), app.CreateInput{
		BusinessID:        f.businessID,
		ExternalReference: externalReference,
		Amount:            money.MustFromString("125000.50"),
		ExpiresAt:         reference.Add(24 * time.Hour),
	})
	assertNoError(t, err, "seed reference")
	return result.Reference
}

func (f *spyFixture) webhookInput(externalReference, eventID string) app.RegisterPaymentInput {
	return app.RegisterPaymentInput{
		EventID:           eventID,
		ExternalReference: externalReference,
		Amount:            money.MustFromString("125000.50"),
		TransactionID:     "tx_883721",
		PaidAt:            reference.Add(-time.Minute),
		RawPayload:        []byte(`{}`),
	}
}

// A repository failure must reach the caller unchanged. Swallowing it would make the
// use case report success on a write that never happened.
func TestUseCasesPropagateRepositoryFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("create: the insert fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.references.fail("Create", errDatabase)

		_, err := f.create.Execute(ctx, app.CreateInput{
			BusinessID:        f.businessID,
			ExternalReference: "INV-1",
			Amount:            money.MustFromString("10.00"),
			ExpiresAt:         reference.Add(time.Hour),
		})
		if !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})

	t.Run("create: the business lookup fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.businesses.err = errDatabase

		_, err := f.create.Execute(ctx, app.CreateInput{
			BusinessID:        f.businessID,
			ExternalReference: "INV-1",
			Amount:            money.MustFromString("10.00"),
			ExpiresAt:         reference.Add(time.Hour),
		})
		if !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
		// Not to be confused with a business that simply does not exist, which is a
		// 422 the client can act on.
		if errors.Is(err, app.ErrBusinessNotFound) {
			t.Error("an infrastructure failure must not be reported as a missing business")
		}
	})

	t.Run("get: the read fails", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")
		f.references.fail("GetByID", errDatabase)

		if _, err := f.get.Execute(ctx, created.ID); !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})

	t.Run("list: the query fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.references.fail("List", errDatabase)

		if _, err := f.list.Execute(ctx, app.Filter{}); !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})

	t.Run("cancel: the lock fails", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")
		f.references.fail("LockByID", errDatabase)

		if _, err := f.cancel.Execute(ctx, created.ID); !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})

	t.Run("cancel: the guarded update fails", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")
		f.references.fail("UpdateWithStatusGuard", domain.ErrConcurrentUpdate)

		_, err := f.cancel.Execute(ctx, created.ID)
		if !errors.Is(err, domain.ErrConcurrentUpdate) {
			t.Fatalf("got error %v, want ErrConcurrentUpdate", err)
		}
		// The caller is told to retry rather than being handed a reference that
		// claims to be cancelled while the row is not.
		if f.references.PaymentReferenceRepository == nil {
			t.Fatal("unexpected fixture state")
		}
		stored, err := f.references.PaymentReferenceRepository.GetByID(ctx, created.ID)
		assertNoError(t, err, "read back")
		if stored.Status != domain.StatusPending {
			t.Errorf("stored status: got %q, want pending", stored.Status)
		}
	})

	t.Run("webhook: recording the event fails", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")
		f.events.fail("Insert", errDatabase)

		_, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1"))
		if !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
		// Nothing may have been applied: the provider has to retry, and it only will
		// if we answer 5xx, which is what propagating this error produces.
		stored, err := f.references.PaymentReferenceRepository.GetByID(ctx, created.ID)
		assertNoError(t, err, "read back")
		if stored.Status != domain.StatusPending {
			t.Errorf("stored status: got %q, want pending", stored.Status)
		}
	})

	t.Run("webhook: stamping the outcome fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.seed(t, "INV-1")
		f.events.fail("Finalize", errDatabase)

		_, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1"))
		if !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})

	t.Run("webhook: the guarded update fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.seed(t, "INV-1")
		f.references.fail("UpdateWithStatusGuard", errDatabase)

		_, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1"))
		if !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
		// The outcome was never stamped, so nothing claims the payment was applied.
		if f.events.sawTransaction("Finalize") {
			t.Error("the outcome must not be stamped after the update failed")
		}
	})

	t.Run("webhook: reading the original outcome of a redelivery fails", func(t *testing.T) {
		f := newSpyFixture(t)
		f.seed(t, "INV-1")
		if _, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1")); err != nil {
			t.Fatalf("first delivery: %v", err)
		}

		f.events.fail("GetByEventID", errDatabase)
		if _, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1")); !errors.Is(err, errDatabase) {
			t.Fatalf("got error %v, want the database failure", err)
		}
	})
}

// The transaction boundary is part of the contract of these two use cases, not an
// implementation detail: without it a payment could be recorded without the reference
// moving, or the other way round.
func TestCancelAndWebhookRunInOneTransaction(t *testing.T) {
	ctx := context.Background()

	t.Run("cancel", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")

		if _, err := f.cancel.Execute(ctx, created.ID); err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if got := f.tx.transactions(); got != 1 {
			t.Errorf("opened %d transactions, want exactly 1", got)
		}
		for _, method := range []string{"LockByID", "UpdateWithStatusGuard"} {
			if !f.references.sawTransaction(method) {
				t.Errorf("%s ran outside the transaction", method)
			}
		}
	})

	t.Run("webhook", func(t *testing.T) {
		f := newSpyFixture(t)
		f.seed(t, "INV-1")

		if _, err := f.registerPayment.Execute(ctx, f.webhookInput("INV-1", "evt_1")); err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if got := f.tx.transactions(); got != 1 {
			t.Errorf("opened %d transactions, want exactly 1", got)
		}
		if !f.events.sawTransaction("Insert") {
			t.Error("the event insert ran outside the transaction")
		}
		if !f.events.sawTransaction("Finalize") {
			t.Error("stamping the outcome ran outside the transaction")
		}
		for _, method := range []string{"LockByExternalReference", "UpdateWithStatusGuard"} {
			if !f.references.sawTransaction(method) {
				t.Errorf("%s ran outside the transaction", method)
			}
		}
	})

	// Reads are deliberately not wrapped in a transaction: a single statement is
	// already atomic, and an extra BEGIN/COMMIT per read is pure overhead.
	t.Run("reads open no transaction", func(t *testing.T) {
		f := newSpyFixture(t)
		created := f.seed(t, "INV-1")

		if _, err := f.get.Execute(ctx, created.ID); err != nil {
			t.Fatalf("get: %v", err)
		}
		if _, err := f.list.Execute(ctx, app.Filter{}); err != nil {
			t.Fatalf("list: %v", err)
		}

		if got := f.tx.transactions(); got != 0 {
			t.Errorf("opened %d transactions for reads, want 0", got)
		}
	})
}
