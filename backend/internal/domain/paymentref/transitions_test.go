package paymentref

import (
	"errors"
	"testing"
	"time"

	"paymentlinks/internal/domain/money"
)

// This is the exhaustive table of the state machine at the entity level: every
// starting state crossed with every operation, valid and invalid.
//
// TestStatusCanTransitionTo already covers the Status -> Status matrix in isolation.
// What this adds is the behaviour of the operations that drive it: which error comes
// out, and the property that matters most in a payments system, that a rejected
// transition leaves the entity completely untouched.

// startingState builds a reference in the wanted state, using only the domain API.
type startingState struct {
	name  string
	build func(t *testing.T) *PaymentReference
	// now is the instant the operation is evaluated at.
	now time.Time
}

func buildPending(t *testing.T) *PaymentReference {
	t.Helper()

	ref, err := New(NewParams{
		BusinessID:        1,
		ExternalReference: "INV-1",
		Amount:            money.MustFromString("100.00"),
		ExpiresAt:         tomorrow,
		Now:               now,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return ref
}

func buildPaid(t *testing.T) *PaymentReference {
	t.Helper()

	ref := buildPending(t)
	if err := ref.MarkAsPaid("tx_1", now, now); err != nil {
		t.Fatalf("MarkAsPaid: %v", err)
	}
	return ref
}

func buildCancelled(t *testing.T) *PaymentReference {
	t.Helper()

	ref := buildPending(t)
	if err := ref.Cancel(now); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	return ref
}

func buildExpired(t *testing.T) *PaymentReference {
	t.Helper()

	ref := buildPending(t)
	if err := ref.Expire(afterExpiry); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	return ref
}

var afterExpiry = tomorrow.Add(time.Hour)

var startingStates = []startingState{
	{name: "pending", build: buildPending, now: now},
	// Stored as pending but past its date: the state the periodic job has not caught
	// up with yet, and the one most likely to be handled inconsistently.
	{name: "pending past due", build: buildPending, now: afterExpiry},
	{name: "expired", build: buildExpired, now: afterExpiry},
	{name: "paid", build: buildPaid, now: now},
	{name: "cancelled", build: buildCancelled, now: now},
}

// transition is one cell of the table.
type transition struct {
	// wantErr is nil when the operation is allowed.
	wantErr error
	// wantStatus is the status afterwards, whether or not the operation succeeded.
	wantStatus Status
	// wantReview is true when the operation must flag the reference for review.
	wantReview bool
}

func TestCancelTransitions(t *testing.T) {
	expected := map[string]transition{
		"pending":          {wantErr: nil, wantStatus: StatusCancelled},
		"pending past due": {wantErr: ErrCannotCancelExpired, wantStatus: StatusPending},
		"expired":          {wantErr: ErrCannotCancelExpired, wantStatus: StatusExpired},
		"paid":             {wantErr: ErrCannotCancelPaid, wantStatus: StatusPaid},
		// Cancelling twice is idempotent by decision: cancel expresses a desired end
		// state, so a client retry reports success without changing anything.
		"cancelled": {wantErr: nil, wantStatus: StatusCancelled},
	}

	for _, state := range startingStates {
		t.Run(state.name, func(t *testing.T) {
			want := expected[state.name]
			ref := state.build(t)
			before := *ref

			err := ref.Cancel(state.now)

			assertTransition(t, ref, before, err, want)
			if want.wantErr == nil && want.wantStatus == StatusCancelled && ref.CancelledAt == nil {
				t.Error("a cancelled reference must carry cancelled_at")
			}
		})
	}
}

func TestMarkAsPaidTransitions(t *testing.T) {
	expected := map[string]transition{
		"pending": {wantErr: nil, wantStatus: StatusPaid},
		// The agreed assumption: the money arrived, so it is recorded, and flagged so
		// a human decides whether to honour or refund it.
		"pending past due": {wantErr: nil, wantStatus: StatusPaid, wantReview: true},
		"expired":          {wantErr: nil, wantStatus: StatusPaid, wantReview: true},
		"paid":             {wantErr: ErrAlreadyPaid, wantStatus: StatusPaid},
		"cancelled":        {wantErr: ErrCannotPayCancelled, wantStatus: StatusCancelled},
	}

	for _, state := range startingStates {
		t.Run(state.name, func(t *testing.T) {
			want := expected[state.name]
			ref := state.build(t)
			before := *ref

			err := ref.MarkAsPaid("tx_new", state.now, state.now)

			assertTransition(t, ref, before, err, want)

			if want.wantErr != nil {
				return
			}
			if ref.PaidAt == nil {
				t.Error("a paid reference must carry paid_at")
			}
			if ref.PaidTransactionID != "tx_new" {
				t.Errorf("transaction_id: got %q, want tx_new", ref.PaidTransactionID)
			}
		})
	}
}

func TestExpireTransitions(t *testing.T) {
	expected := map[string]transition{
		// Not past its date yet, so there is nothing to expire.
		"pending":          {wantErr: ErrNotYetExpired, wantStatus: StatusPending},
		"pending past due": {wantErr: nil, wantStatus: StatusExpired},
		"expired":          {wantErr: ErrInvalidTransition, wantStatus: StatusExpired},
		"paid":             {wantErr: ErrInvalidTransition, wantStatus: StatusPaid},
		"cancelled":        {wantErr: ErrInvalidTransition, wantStatus: StatusCancelled},
	}

	for _, state := range startingStates {
		t.Run(state.name, func(t *testing.T) {
			want := expected[state.name]
			ref := state.build(t)
			before := *ref

			err := ref.Expire(state.now)

			assertTransition(t, ref, before, err, want)
		})
	}
}

// assertTransition checks the outcome and, when the operation was rejected, that the
// entity was left exactly as it was. A partially applied rejected transition is the
// worst failure mode here: it would leave a reference nobody can explain.
func assertTransition(t *testing.T, ref *PaymentReference, before PaymentReference, err error, want transition) {
	t.Helper()

	if want.wantErr == nil {
		if err != nil {
			t.Fatalf("got error %v, want the operation to be allowed", err)
		}
	} else if !errors.Is(err, want.wantErr) {
		t.Fatalf("got error %v, want %v", err, want.wantErr)
	}

	if ref.Status != want.wantStatus {
		t.Errorf("status: got %q, want %q", ref.Status, want.wantStatus)
	}
	if ref.RequiresReview != want.wantReview {
		t.Errorf("requires_review: got %v, want %v", ref.RequiresReview, want.wantReview)
	}

	if want.wantErr == nil {
		return
	}

	// Rejected: nothing may have moved.
	switch {
	case ref.UpdatedAt != before.UpdatedAt:
		t.Error("a rejected transition must not touch updated_at")
	case ref.PaidAt != before.PaidAt:
		t.Error("a rejected transition must not touch paid_at")
	case ref.CancelledAt != before.CancelledAt:
		t.Error("a rejected transition must not touch cancelled_at")
	case ref.PaidTransactionID != before.PaidTransactionID:
		t.Error("a rejected transition must not touch the transaction id")
	case ref.ReviewReason != before.ReviewReason:
		t.Error("a rejected transition must not touch the review reason")
	}
}

// A rejected payment must not even record the transaction id, which is the detail
// most likely to leak through when the branches are written by hand.
func TestRejectedPaymentLeavesNoTrace(t *testing.T) {
	ref := buildCancelled(t)

	if err := ref.MarkAsPaid("tx_leak", now, now); !errors.Is(err, ErrCannotPayCancelled) {
		t.Fatalf("got error %v, want ErrCannotPayCancelled", err)
	}
	if ref.PaidTransactionID != "" || ref.PaidAt != nil {
		t.Errorf("got transaction_id=%q paid_at=%v, want neither", ref.PaidTransactionID, ref.PaidAt)
	}
}

// Terminal states are terminal: no operation moves a paid or cancelled reference.
func TestTerminalStatesAreClosed(t *testing.T) {
	for _, build := range []func(*testing.T) *PaymentReference{buildPaid, buildCancelled} {
		ref := build(t)
		initial := ref.Status

		_ = ref.Cancel(afterExpiry)
		_ = ref.MarkAsPaid("tx_x", afterExpiry, afterExpiry)
		_ = ref.Expire(afterExpiry)

		if ref.Status != initial {
			t.Errorf("a %s reference ended up %s after every operation was attempted", initial, ref.Status)
		}
	}
}
