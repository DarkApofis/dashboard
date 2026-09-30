package paymentref_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
)

const paidTransaction = "tx_883721"

func webhookInput(externalReference, eventID, amount string) app.RegisterPaymentInput {
	return app.RegisterPaymentInput{
		EventID:           eventID,
		ExternalReference: externalReference,
		Amount:            money.MustFromString(amount),
		TransactionID:     paidTransaction,
		PaidAt:            reference.Add(-time.Minute),
		RawPayload:        []byte(`{"event_id":"` + eventID + `"}`),
	}
}

// seedReference creates a pending reference of 125000.50 and returns it.
func (f *fixture) seedReference(t *testing.T, externalReference string) *domain.PaymentReference {
	t.Helper()

	created, err := f.create.Execute(context.Background(), f.createInput(externalReference))
	if err != nil {
		t.Fatalf("create %s: %v", externalReference, err)
	}
	return created.Reference
}

func (f *fixture) storedReference(t *testing.T, id int64) *domain.PaymentReference {
	t.Helper()

	result, err := f.get.Execute(context.Background(), id)
	if err != nil {
		t.Fatalf("get %d: %v", id, err)
	}
	return result.Reference
}

func TestRegisterPaymentApplies(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created := f.seedReference(t, "INV-10023")

	result, err := f.registerPayment.Execute(ctx, webhookInput("INV-10023", "evt_1", "125000.50"))
	if err != nil {
		t.Fatalf("Execute: unexpected error %v", err)
	}

	if result.Outcome != domain.OutcomeApplied {
		t.Errorf("outcome: got %q, want applied", result.Outcome)
	}
	if !result.Applied || result.Duplicate {
		t.Errorf("got applied=%v duplicate=%v, want true and false", result.Applied, result.Duplicate)
	}

	stored := f.storedReference(t, created.ID)
	if stored.Status != domain.StatusPaid {
		t.Errorf("status: got %q, want paid", stored.Status)
	}
	if stored.PaidTransactionID != paidTransaction {
		t.Errorf("transaction_id: got %q, want %q", stored.PaidTransactionID, paidTransaction)
	}
	if stored.PaidAt == nil || !stored.PaidAt.Equal(reference.Add(-time.Minute)) {
		t.Errorf("paid_at: got %v, want the timestamp the provider sent", stored.PaidAt)
	}
	if stored.RequiresReview {
		t.Error("a payment on time and for the right amount must not require review")
	}
}

// An amount is compared exactly: "125000.5" and "125000.50" are the same money, and
// one cent of difference is a mismatch. This is the whole reason the wire value never
// passes through a float.
func TestRegisterPaymentComparesAmountsExactly(t *testing.T) {
	ctx := context.Background()

	t.Run("trailing zero is the same amount", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")

		result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.5"))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Outcome != domain.OutcomeApplied {
			t.Fatalf("outcome: got %q, want applied", result.Outcome)
		}
		if f.storedReference(t, created.ID).Status != domain.StatusPaid {
			t.Error("the reference must be paid")
		}
	})

	t.Run("one cent off is a mismatch", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")

		result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.51"))
		if err != nil {
			t.Fatalf("Execute: unexpected error %v (a business rejection is not an error)", err)
		}
		if result.Outcome != domain.OutcomeAmountMismatch {
			t.Errorf("outcome: got %q, want amount_mismatch", result.Outcome)
		}
		if result.Applied {
			t.Error("a mismatched amount must not be applied")
		}

		stored := f.storedReference(t, created.ID)
		if stored.Status != domain.StatusPending {
			t.Errorf("status: got %q, want pending", stored.Status)
		}
		if !stored.RequiresReview || stored.ReviewReason != domain.ReviewReasonAmountMismatch {
			t.Errorf("got requires_review=%v reason=%q", stored.RequiresReview, stored.ReviewReason)
		}
	})
}

// The provider redelivers whenever it does not get a response. Redelivery must not
// apply anything twice and must report the original outcome.
func TestRegisterPaymentIsIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created := f.seedReference(t, "INV-10023")

	first, err := f.registerPayment.Execute(ctx, webhookInput("INV-10023", "evt_1", "125000.50"))
	if err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	firstUpdatedAt := f.storedReference(t, created.ID).UpdatedAt

	// Time moves on, the provider retries the very same event.
	f.clock.Advance(time.Hour)
	second, err := f.registerPayment.Execute(ctx, webhookInput("INV-10023", "evt_1", "125000.50"))
	if err != nil {
		t.Fatalf("redelivery: unexpected error %v", err)
	}

	if !second.Duplicate {
		t.Error("a redelivery must be reported as a duplicate")
	}
	if second.Applied {
		t.Error("a redelivery must not apply anything again")
	}
	if second.Outcome != first.Outcome {
		t.Errorf("outcome: got %q, want the original %q", second.Outcome, first.Outcome)
	}
	if f.events.Insertions() != 1 {
		t.Errorf("recorded %d events, want 1: the unique event_id must reject the second", f.events.Insertions())
	}

	stored := f.storedReference(t, created.ID)
	if !stored.UpdatedAt.Equal(firstUpdatedAt) {
		t.Error("a redelivery must not touch the reference")
	}
	if second.Reference == nil || second.Reference.ID != created.ID {
		t.Error("the replayed result must still point at the reference")
	}
}

func TestRegisterPaymentUnknownReference(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	result, err := f.registerPayment.Execute(ctx, webhookInput("INV-DOES-NOT-EXIST", "evt_1", "10.00"))

	// Reported as not found so the provider retries. It is the only rejection that can
	// succeed later, because the reference may be created in between.
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got error %v, want ErrNotFound", err)
	}
	if result.Outcome != domain.OutcomeReferenceNotFound {
		t.Errorf("outcome: got %q, want reference_not_found", result.Outcome)
	}
	if result.Reference != nil {
		t.Error("there is no reference to report")
	}

	// Recorded anyway: it is the evidence for whoever reconciles.
	stored, err := f.events.GetByEventID(ctx, "evt_1")
	if err != nil {
		t.Fatalf("the event must have been recorded: %v", err)
	}
	if stored.Outcome != domain.OutcomeReferenceNotFound {
		t.Errorf("stored outcome: got %q", stored.Outcome)
	}
}

// The regression this guards: a payment that arrived before its reference existed used
// to be lost for good. The 200 told the provider to stop retrying, the event_id was
// burned, and the redelivery replayed "not found" while the reference sat unpaid.
func TestRegisterPaymentAppliesAfterTheReferenceAppears(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	// 1. The provider confirms the payment before the business registers the charge.
	if _, err := f.registerPayment.Execute(ctx, webhookInput("INV-LATE", "evt_race", "125000.50")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("first delivery: got error %v, want ErrNotFound", err)
	}

	// 2. The business creates it a moment later.
	created := f.seedReference(t, "INV-LATE")

	// 3. The provider redelivers the very same event_id.
	result, err := f.registerPayment.Execute(ctx, webhookInput("INV-LATE", "evt_race", "125000.50"))
	if err != nil {
		t.Fatalf("redelivery: unexpected error %v", err)
	}
	if result.Outcome != domain.OutcomeApplied || !result.Applied {
		t.Fatalf("got outcome %q applied=%v, want applied", result.Outcome, result.Applied)
	}

	stored := f.storedReference(t, created.ID)
	if stored.Status != domain.StatusPaid {
		t.Errorf("status: got %q, want paid", stored.Status)
	}
	if stored.PaidTransactionID != paidTransaction {
		t.Errorf("transaction_id: got %q", stored.PaidTransactionID)
	}

	// Still one row for the event: it is reused, not inserted again.
	if f.events.Insertions() != 1 {
		t.Errorf("recorded %d events, want 1", f.events.Insertions())
	}
	event, err := f.events.GetByEventID(ctx, "evt_race")
	if err != nil {
		t.Fatalf("GetByEventID: %v", err)
	}
	if event.Outcome != domain.OutcomeApplied {
		t.Errorf("the stored outcome must converge to applied, got %q", event.Outcome)
	}

	// And a further redelivery is an ordinary duplicate again.
	again, err := f.registerPayment.Execute(ctx, webhookInput("INV-LATE", "evt_race", "125000.50"))
	if err != nil {
		t.Fatalf("third delivery: %v", err)
	}
	if !again.Duplicate || again.Applied {
		t.Errorf("got duplicate=%v applied=%v, want true and false", again.Duplicate, again.Applied)
	}
}

func TestRegisterPaymentCancelledReference(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created := f.seedReference(t, "INV-1")
	if _, err := f.cancel.Execute(ctx, created.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.50"))
	if err != nil {
		t.Fatalf("Execute: unexpected error %v", err)
	}
	if result.Outcome != domain.OutcomeConflictCancelled {
		t.Errorf("outcome: got %q, want conflict_cancelled", result.Outcome)
	}
	if result.Applied {
		t.Error("a cancelled reference must not be paid")
	}

	stored := f.storedReference(t, created.ID)
	if stored.Status != domain.StatusCancelled {
		t.Errorf("status: got %q, want cancelled", stored.Status)
	}
	if stored.PaidAt != nil || stored.PaidTransactionID != "" {
		t.Errorf("nothing about the payment may be stored: paid_at=%v tx=%q", stored.PaidAt, stored.PaidTransactionID)
	}
	if !stored.RequiresReview || stored.ReviewReason != domain.ReviewReasonPaymentForCancelled {
		t.Errorf("got requires_review=%v reason=%q", stored.RequiresReview, stored.ReviewReason)
	}
}

// The agreed assumption: an expired reference is still paid, because the money did
// arrive, and it is flagged so a human decides whether to honour or refund it.
func TestRegisterPaymentExpiredReference(t *testing.T) {
	ctx := context.Background()

	t.Run("expiration not yet materialised", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")
		f.clock.Advance(48 * time.Hour)

		result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.50"))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Outcome != domain.OutcomePaidAfterExpiration {
			t.Errorf("outcome: got %q, want paid_after_expiration", result.Outcome)
		}
		if !result.Applied {
			t.Error("the payment must be applied")
		}

		stored := f.storedReference(t, created.ID)
		if stored.Status != domain.StatusPaid {
			t.Errorf("status: got %q, want paid", stored.Status)
		}
		if !stored.RequiresReview || stored.ReviewReason != domain.ReviewReasonPaidAfterExpiration {
			t.Errorf("got requires_review=%v reason=%q", stored.RequiresReview, stored.ReviewReason)
		}
	})

	t.Run("expiration already written by the job", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")
		f.clock.Advance(48 * time.Hour)
		if _, err := f.references.ExpireDue(ctx, f.clock.Now(), 10); err != nil {
			t.Fatalf("ExpireDue: %v", err)
		}
		if f.storedReference(t, created.ID).Status != domain.StatusExpired {
			t.Fatal("the reference should be stored as expired")
		}

		result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.50"))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Outcome != domain.OutcomePaidAfterExpiration || !result.Applied {
			t.Errorf("got outcome %q applied=%v", result.Outcome, result.Applied)
		}
		if f.storedReference(t, created.ID).Status != domain.StatusPaid {
			t.Error("an expired reference must still be payable")
		}
	})
}

func TestRegisterPaymentOnAlreadyPaidReference(t *testing.T) {
	ctx := context.Background()

	t.Run("same transaction under a new event id changes nothing", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")
		if _, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.50")); err != nil {
			t.Fatalf("first delivery: %v", err)
		}
		paidAtBefore := *f.storedReference(t, created.ID).PaidAt

		f.clock.Advance(time.Hour)
		result, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_2", "125000.50"))
		if err != nil {
			t.Fatalf("second delivery: %v", err)
		}
		if result.Outcome != domain.OutcomeDuplicate {
			t.Errorf("outcome: got %q, want duplicate", result.Outcome)
		}
		if result.Applied {
			t.Error("nothing was applied")
		}

		stored := f.storedReference(t, created.ID)
		if !stored.PaidAt.Equal(paidAtBefore) {
			t.Error("paid_at must not move")
		}
		if stored.RequiresReview {
			t.Error("the same payment arriving twice is not worth a human's time")
		}
		if f.events.Insertions() != 2 {
			t.Errorf("recorded %d events, want 2: a different event_id is still a new delivery", f.events.Insertions())
		}
	})

	t.Run("a different transaction is a conflict and does not overwrite", func(t *testing.T) {
		f := newFixture(t)
		created := f.seedReference(t, "INV-1")
		if _, err := f.registerPayment.Execute(ctx, webhookInput("INV-1", "evt_1", "125000.50")); err != nil {
			t.Fatalf("first delivery: %v", err)
		}

		other := webhookInput("INV-1", "evt_2", "125000.50")
		other.TransactionID = "tx_OTHER"

		result, err := f.registerPayment.Execute(ctx, other)
		if err != nil {
			t.Fatalf("second delivery: %v", err)
		}
		if result.Outcome != domain.OutcomeConflictOtherTransaction {
			t.Errorf("outcome: got %q, want conflict_other_transaction", result.Outcome)
		}

		stored := f.storedReference(t, created.ID)
		if stored.PaidTransactionID != paidTransaction {
			t.Errorf("the first transaction must not be overwritten: got %q", stored.PaidTransactionID)
		}
		if !stored.RequiresReview || stored.ReviewReason != domain.ReviewReasonConflictingTransaction {
			t.Errorf("a possible double charge must be flagged: requires_review=%v reason=%q",
				stored.RequiresReview, stored.ReviewReason)
		}
	})
}

// A malformed payload is refused before anything is recorded: without an event_id
// there is no key to record it under.
func TestRegisterPaymentRejectsMalformedPayloads(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name   string
		mutate func(*app.RegisterPaymentInput)
	}{
		{"missing event id", func(in *app.RegisterPaymentInput) { in.EventID = "  " }},
		{"missing external reference", func(in *app.RegisterPaymentInput) { in.ExternalReference = "" }},
		{"missing transaction id", func(in *app.RegisterPaymentInput) { in.TransactionID = "" }},
		{"zero amount", func(in *app.RegisterPaymentInput) { in.Amount = money.Zero() }},
		{"missing paid_at", func(in *app.RegisterPaymentInput) { in.PaidAt = time.Time{} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.seedReference(t, "INV-1")

			input := webhookInput("INV-1", "evt_1", "125000.50")
			tc.mutate(&input)

			_, err := f.registerPayment.Execute(ctx, input)
			if !errors.Is(err, domain.ErrInvalidWebhookPayload) {
				t.Fatalf("got error %v, want ErrInvalidWebhookPayload", err)
			}
			if f.events.Insertions() != 0 {
				t.Errorf("recorded %d events, want 0", f.events.Insertions())
			}
		})
	}
}
