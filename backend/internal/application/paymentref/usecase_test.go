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

var reference = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

type fixture struct {
	references *inmemory.PaymentReferenceRepository
	businesses *inmemory.BusinessRepository
	events     *inmemory.WebhookEventRepository
	clock      *clock.Fixed
	businessID int64

	create          *app.CreateReference
	get             *app.GetReference
	list            *app.ListReferences
	cancel          *app.CancelReference
	registerPayment *app.RegisterPayment
}

// newFixture wires the real use cases against the in-memory ports: the same code
// path production takes, with persistence and the clock swapped out.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	references := inmemory.NewPaymentReferenceRepository()
	businesses := inmemory.NewBusinessRepository()
	events := inmemory.NewWebhookEventRepository()
	tx := inmemory.NewTxManager()
	fixed := clock.NewFixed(reference)
	ids := businesses.Seed("Acme Logistics")

	return &fixture{
		references:      references,
		businesses:      businesses,
		events:          events,
		clock:           fixed,
		businessID:      ids[0],
		create:          app.NewCreateReference(references, businesses, fixed),
		get:             app.NewGetReference(references, fixed),
		list:            app.NewListReferences(references, fixed),
		cancel:          app.NewCancelReference(tx, references, fixed),
		registerPayment: app.NewRegisterPayment(tx, references, events, fixed),
	}
}

func (f *fixture) createInput(externalReference string) app.CreateInput {
	return app.CreateInput{
		BusinessID:        f.businessID,
		ExternalReference: externalReference,
		Amount:            money.MustFromString("125000.50"),
		Description:       "Factura " + externalReference,
		ExpiresAt:         reference.Add(24 * time.Hour),
	}
}

func TestCreateReference(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a pending reference", func(t *testing.T) {
		f := newFixture(t)

		result, err := f.create.Execute(ctx, f.createInput("INV-10023"))
		if err != nil {
			t.Fatalf("Execute: unexpected error %v", err)
		}
		if result.Reference.ID == 0 {
			t.Error("the repository must assign an id")
		}
		if result.Reference.Status != domain.StatusPending {
			t.Errorf("status: got %q, want pending", result.Reference.Status)
		}
		if result.Reference.Amount.String() != "125000.50" {
			t.Errorf("amount: got %q", result.Reference.Amount.String())
		}
	})

	t.Run("rejects an unknown business", func(t *testing.T) {
		f := newFixture(t)
		input := f.createInput("INV-1")
		input.BusinessID = 999

		_, err := f.create.Execute(ctx, input)
		if !errors.Is(err, app.ErrBusinessNotFound) {
			t.Fatalf("got error %v, want ErrBusinessNotFound", err)
		}
	})

	t.Run("rejects a duplicate external reference", func(t *testing.T) {
		f := newFixture(t)
		if _, err := f.create.Execute(ctx, f.createInput("INV-10023")); err != nil {
			t.Fatalf("first create: %v", err)
		}

		_, err := f.create.Execute(ctx, f.createInput("INV-10023"))
		if !errors.Is(err, domain.ErrDuplicateReference) {
			t.Fatalf("got error %v, want ErrDuplicateReference", err)
		}
	})

	t.Run("rejects an expiration in the past", func(t *testing.T) {
		f := newFixture(t)
		input := f.createInput("INV-1")
		input.ExpiresAt = reference.Add(-time.Hour)

		_, err := f.create.Execute(ctx, input)
		if !errors.Is(err, domain.ErrExpirationNotInFuture) {
			t.Fatalf("got error %v, want ErrExpirationNotInFuture", err)
		}
	})

	t.Run("rejects a non positive amount", func(t *testing.T) {
		f := newFixture(t)
		input := f.createInput("INV-1")
		input.Amount = money.Zero()

		_, err := f.create.Execute(ctx, input)
		if !errors.Is(err, domain.ErrAmountNotPositive) {
			t.Fatalf("got error %v, want ErrAmountNotPositive", err)
		}
	})
}

func TestGetReference(t *testing.T) {
	ctx := context.Background()

	t.Run("reports a past due pending reference as expired", func(t *testing.T) {
		f := newFixture(t)
		created, err := f.create.Execute(ctx, f.createInput("INV-1"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// The periodic job has not run, so the stored status is still pending.
		f.clock.Advance(48 * time.Hour)

		result, err := f.get.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("Execute: unexpected error %v", err)
		}
		if result.Reference.Status != domain.StatusPending {
			t.Errorf("stored status: got %q, want pending (a read must not write)", result.Reference.Status)
		}
		if got := result.EffectiveStatus(); got != domain.StatusExpired {
			t.Errorf("effective status: got %q, want expired", got)
		}
	})

	t.Run("missing reference is not found", func(t *testing.T) {
		f := newFixture(t)

		if _, err := f.get.Execute(ctx, 4242); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want ErrNotFound", err)
		}
	})
}

func TestListReferences(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	for _, external := range []string{"INV-1", "INV-2", "INV-3"} {
		if _, err := f.create.Execute(ctx, f.createInput(external)); err != nil {
			t.Fatalf("create %s: %v", external, err)
		}
	}
	if _, err := f.cancel.Execute(ctx, 2); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	t.Run("filters by status", func(t *testing.T) {
		cancelled := domain.StatusCancelled
		result, err := f.list.Execute(ctx, app.Filter{Status: &cancelled})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Page.Total != 1 || len(result.Page.Items) != 1 {
			t.Fatalf("got total=%d items=%d, want 1 and 1", result.Page.Total, len(result.Page.Items))
		}
		if result.Page.Items[0].ExternalReference != "INV-2" {
			t.Errorf("got %q, want INV-2", result.Page.Items[0].ExternalReference)
		}
	})

	t.Run("filters by external reference", func(t *testing.T) {
		external := "INV-3"
		result, err := f.list.Execute(ctx, app.Filter{ExternalReference: &external})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Page.Total != 1 {
			t.Fatalf("total: got %d, want 1", result.Page.Total)
		}
	})

	t.Run("filters by business", func(t *testing.T) {
		other := int64(4242)
		result, err := f.list.Execute(ctx, app.Filter{BusinessID: &other})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Page.Total != 0 || len(result.Page.Items) != 0 {
			t.Errorf("an unknown business must match nothing, got total=%d", result.Page.Total)
		}
	})

	t.Run("caps the page size and reports the full total", func(t *testing.T) {
		result, err := f.list.Execute(ctx, app.Filter{Limit: 2})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if len(result.Page.Items) != 2 {
			t.Errorf("items: got %d, want 2", len(result.Page.Items))
		}
		if result.Page.Total != 3 {
			t.Errorf("total must ignore pagination: got %d, want 3", result.Page.Total)
		}
	})

	t.Run("clamps a limit above the maximum", func(t *testing.T) {
		result, err := f.list.Execute(ctx, app.Filter{Limit: 5000})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Page.Limit != app.MaxLimit {
			t.Errorf("limit: got %d, want %d", result.Page.Limit, app.MaxLimit)
		}
	})

	t.Run("expired references surface without the job having run", func(t *testing.T) {
		expiredFixture := newFixture(t)
		if _, err := expiredFixture.create.Execute(ctx, expiredFixture.createInput("INV-OLD")); err != nil {
			t.Fatalf("create: %v", err)
		}
		expiredFixture.clock.Advance(48 * time.Hour)

		expired := domain.StatusExpired
		result, err := expiredFixture.list.Execute(ctx, app.Filter{Status: &expired})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Page.Total != 1 {
			t.Fatalf("total: got %d, want 1", result.Page.Total)
		}
	})
}

func TestCancelReference(t *testing.T) {
	ctx := context.Background()

	t.Run("cancels a pending reference", func(t *testing.T) {
		f := newFixture(t)
		created, err := f.create.Execute(ctx, f.createInput("INV-1"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		result, err := f.cancel.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("Execute: unexpected error %v", err)
		}
		if result.Reference.Status != domain.StatusCancelled {
			t.Errorf("status: got %q, want cancelled", result.Reference.Status)
		}

		// The cancellation must have been persisted, not only applied in memory.
		stored, err := f.get.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if stored.Reference.Status != domain.StatusCancelled {
			t.Errorf("stored status: got %q, want cancelled", stored.Reference.Status)
		}
	})

	t.Run("cancelling twice succeeds and changes nothing", func(t *testing.T) {
		f := newFixture(t)
		created, err := f.create.Execute(ctx, f.createInput("INV-1"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		first, err := f.cancel.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("first cancel: %v", err)
		}

		f.clock.Advance(time.Hour)
		second, err := f.cancel.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("second cancel: got error %v, want nil (idempotent)", err)
		}
		if !second.Reference.CancelledAt.Equal(*first.Reference.CancelledAt) {
			t.Error("a repeated cancel must not move cancelled_at")
		}
	})

	t.Run("a paid reference cannot be cancelled", func(t *testing.T) {
		f := newFixture(t)
		created, err := f.create.Execute(ctx, f.createInput("INV-1"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Simulate the webhook until that use case exists.
		paid := created.Reference
		if err := paid.MarkAsPaid("tx_1", reference, reference); err != nil {
			t.Fatalf("MarkAsPaid: %v", err)
		}
		if err := f.references.UpdateWithStatusGuard(ctx, paid, domain.StatusPending); err != nil {
			t.Fatalf("update: %v", err)
		}

		_, err = f.cancel.Execute(ctx, created.Reference.ID)
		if !errors.Is(err, domain.ErrCannotCancelPaid) {
			t.Fatalf("got error %v, want ErrCannotCancelPaid", err)
		}
	})

	t.Run("an expired reference cannot be cancelled and its expiration is stored", func(t *testing.T) {
		f := newFixture(t)
		created, err := f.create.Execute(ctx, f.createInput("INV-1"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		f.clock.Advance(48 * time.Hour)

		_, err = f.cancel.Execute(ctx, created.Reference.ID)
		if !errors.Is(err, domain.ErrCannotCancelExpired) {
			t.Fatalf("got error %v, want ErrCannotCancelExpired", err)
		}

		// The use case held the lock and knew the reference was overdue, so it
		// materialised the expiration instead of leaving the row stale.
		stored, err := f.get.Execute(ctx, created.Reference.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if stored.Reference.Status != domain.StatusExpired {
			t.Errorf("stored status: got %q, want expired", stored.Reference.Status)
		}
	})

	t.Run("a missing reference is not found", func(t *testing.T) {
		f := newFixture(t)

		if _, err := f.cancel.Execute(ctx, 4242); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want ErrNotFound", err)
		}
	})
}
