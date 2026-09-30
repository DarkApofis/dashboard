package paymentref

import (
	"errors"
	"testing"
	"time"

	"paymentlinks/internal/domain/money"
)

var (
	now       = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	tomorrow  = now.Add(24 * time.Hour)
	yesterday = now.Add(-24 * time.Hour)
)

func newPending(t *testing.T) *PaymentReference {
	t.Helper()

	ref, err := New(NewParams{
		BusinessID:        1,
		ExternalReference: "INV-10023",
		Amount:            money.MustFromString("125000.50"),
		Description:       "Factura 10023",
		ExpiresAt:         tomorrow,
		Now:               now,
	})
	if err != nil {
		t.Fatalf("New: unexpected error %v", err)
	}
	return ref
}

func TestNewValidation(t *testing.T) {
	valid := NewParams{
		BusinessID:        1,
		ExternalReference: "INV-10023",
		Amount:            money.MustFromString("10.00"),
		ExpiresAt:         tomorrow,
		Now:               now,
	}

	cases := []struct {
		name    string
		mutate  func(*NewParams)
		wantErr error
	}{
		{"valid", func(*NewParams) {}, nil},
		{"missing business", func(p *NewParams) { p.BusinessID = 0 }, ErrInvalidBusiness},
		{"empty external reference", func(p *NewParams) { p.ExternalReference = "   " }, ErrInvalidExternalReference},
		{"external reference with whitespace", func(p *NewParams) { p.ExternalReference = "INV 1" }, ErrInvalidExternalReference},
		{"zero amount", func(p *NewParams) { p.Amount = money.Zero() }, ErrAmountNotPositive},
		{"expiration in the past", func(p *NewParams) { p.ExpiresAt = yesterday }, ErrExpirationNotInFuture},
		{"expiration missing", func(p *NewParams) { p.ExpiresAt = time.Time{} }, ErrExpirationNotInFuture},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := valid
			tc.mutate(&params)

			ref, err := New(params)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("got error %v, want none", err)
				}
				if ref.Status != StatusPending {
					t.Errorf("new reference status: got %q, want pending", ref.Status)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewTrimsAndNormalizes(t *testing.T) {
	ref, err := New(NewParams{
		BusinessID:        7,
		ExternalReference: "  INV-1  ",
		Amount:            money.MustFromString("1.5"),
		Description:       "  con espacios  ",
		ExpiresAt:         tomorrow,
		Now:               now,
	})
	if err != nil {
		t.Fatalf("New: unexpected error %v", err)
	}

	if ref.ExternalReference != "INV-1" {
		t.Errorf("external reference: got %q", ref.ExternalReference)
	}
	if ref.Description != "con espacios" {
		t.Errorf("description: got %q", ref.Description)
	}
	if ref.Amount.String() != "1.50" {
		t.Errorf("amount: got %q, want 1.50", ref.Amount.String())
	}
}

func TestEffectiveStatusExpiresWithoutBeingStored(t *testing.T) {
	ref := newPending(t)

	if got := ref.EffectiveStatus(now); got != StatusPending {
		t.Errorf("before expiration: got %q, want pending", got)
	}

	// The stored status is still pending because the periodic job has not run,
	// but a read must never report a past-due reference as pending.
	afterExpiry := tomorrow.Add(time.Second)
	if got := ref.EffectiveStatus(afterExpiry); got != StatusExpired {
		t.Errorf("after expiration: got %q, want expired", got)
	}
	if ref.Status != StatusPending {
		t.Errorf("stored status must not change on a read, got %q", ref.Status)
	}
}

func TestCancel(t *testing.T) {
	t.Run("pending is cancelled", func(t *testing.T) {
		ref := newPending(t)

		if err := ref.Cancel(now); err != nil {
			t.Fatalf("Cancel: unexpected error %v", err)
		}
		if ref.Status != StatusCancelled {
			t.Errorf("status: got %q, want cancelled", ref.Status)
		}
		if ref.CancelledAt == nil || !ref.CancelledAt.Equal(now) {
			t.Errorf("cancelled_at: got %v, want %v", ref.CancelledAt, now)
		}
	})

	t.Run("cancelling twice is idempotent", func(t *testing.T) {
		ref := newPending(t)
		if err := ref.Cancel(now); err != nil {
			t.Fatalf("first Cancel: %v", err)
		}
		firstCancelledAt := *ref.CancelledAt

		later := now.Add(time.Hour)
		if err := ref.Cancel(later); err != nil {
			t.Fatalf("second Cancel: got error %v, want nil (idempotent)", err)
		}
		if !ref.CancelledAt.Equal(firstCancelledAt) {
			t.Error("a repeated cancel must not move cancelled_at")
		}
	})

	t.Run("paid cannot be cancelled", func(t *testing.T) {
		ref := newPending(t)
		if err := ref.MarkAsPaid("tx_1", now, now); err != nil {
			t.Fatalf("MarkAsPaid: %v", err)
		}

		if err := ref.Cancel(now); !errors.Is(err, ErrCannotCancelPaid) {
			t.Fatalf("got error %v, want ErrCannotCancelPaid", err)
		}
		if ref.Status != StatusPaid {
			t.Errorf("status must stay paid, got %q", ref.Status)
		}
	})

	t.Run("expired cannot be cancelled", func(t *testing.T) {
		ref := newPending(t)

		err := ref.Cancel(tomorrow.Add(time.Second))
		if !errors.Is(err, ErrCannotCancelExpired) {
			t.Fatalf("got error %v, want ErrCannotCancelExpired", err)
		}
	})
}

func TestMarkPaid(t *testing.T) {
	paidAt := now.Add(-time.Minute)

	t.Run("pending is paid", func(t *testing.T) {
		ref := newPending(t)

		if err := ref.MarkAsPaid("tx_883721", paidAt, now); err != nil {
			t.Fatalf("MarkAsPaid: unexpected error %v", err)
		}
		if ref.Status != StatusPaid {
			t.Errorf("status: got %q, want paid", ref.Status)
		}
		if ref.PaidTransactionID != "tx_883721" {
			t.Errorf("transaction id: got %q", ref.PaidTransactionID)
		}
		if ref.PaidAt == nil || !ref.PaidAt.Equal(paidAt) {
			t.Errorf("paid_at: got %v, want %v", ref.PaidAt, paidAt)
		}
		if ref.RequiresReview {
			t.Error("a payment on time must not require review")
		}
	})

	t.Run("expired is paid and flagged for review", func(t *testing.T) {
		ref := newPending(t)
		afterExpiry := tomorrow.Add(time.Hour)

		if err := ref.MarkAsPaid("tx_late", afterExpiry, afterExpiry); err != nil {
			t.Fatalf("MarkAsPaid: unexpected error %v", err)
		}
		if ref.Status != StatusPaid {
			t.Errorf("status: got %q, want paid", ref.Status)
		}
		if !ref.RequiresReview || ref.ReviewReason != ReviewReasonPaidAfterExpiration {
			t.Errorf("expected review flag %q, got requires_review=%v reason=%q",
				ReviewReasonPaidAfterExpiration, ref.RequiresReview, ref.ReviewReason)
		}
	})

	t.Run("already materialised expired is paid too", func(t *testing.T) {
		ref := newPending(t)
		afterExpiry := tomorrow.Add(time.Hour)
		if err := ref.Expire(afterExpiry); err != nil {
			t.Fatalf("Expire: %v", err)
		}

		if err := ref.MarkAsPaid("tx_late", afterExpiry, afterExpiry); err != nil {
			t.Fatalf("MarkAsPaid on stored expired: unexpected error %v", err)
		}
		if ref.Status != StatusPaid || !ref.RequiresReview {
			t.Errorf("got status %q requires_review=%v", ref.Status, ref.RequiresReview)
		}
	})

	t.Run("cancelled is not paid", func(t *testing.T) {
		ref := newPending(t)
		if err := ref.Cancel(now); err != nil {
			t.Fatalf("Cancel: %v", err)
		}

		if err := ref.MarkAsPaid("tx_1", paidAt, now); !errors.Is(err, ErrCannotPayCancelled) {
			t.Fatalf("got error %v, want ErrCannotPayCancelled", err)
		}
		if ref.Status != StatusCancelled {
			t.Errorf("status must stay cancelled, got %q", ref.Status)
		}
	})

	t.Run("paying twice reports already paid", func(t *testing.T) {
		ref := newPending(t)
		if err := ref.MarkAsPaid("tx_1", paidAt, now); err != nil {
			t.Fatalf("MarkAsPaid: %v", err)
		}

		if err := ref.MarkAsPaid("tx_1", paidAt, now); !errors.Is(err, ErrAlreadyPaid) {
			t.Fatalf("got error %v, want ErrAlreadyPaid", err)
		}
	})

	t.Run("transaction id is required", func(t *testing.T) {
		ref := newPending(t)

		if err := ref.MarkAsPaid("  ", paidAt, now); !errors.Is(err, ErrInvalidTransactionID) {
			t.Fatalf("got error %v, want ErrInvalidTransactionID", err)
		}
	})
}

func TestExpire(t *testing.T) {
	t.Run("past due pending expires", func(t *testing.T) {
		ref := newPending(t)
		afterExpiry := tomorrow.Add(time.Second)

		if err := ref.Expire(afterExpiry); err != nil {
			t.Fatalf("Expire: unexpected error %v", err)
		}
		if ref.Status != StatusExpired {
			t.Errorf("status: got %q, want expired", ref.Status)
		}
	})

	t.Run("not yet due does not expire", func(t *testing.T) {
		ref := newPending(t)

		if err := ref.Expire(now); !errors.Is(err, ErrNotYetExpired) {
			t.Fatalf("got error %v, want ErrNotYetExpired", err)
		}
	})

	t.Run("paid never expires", func(t *testing.T) {
		ref := newPending(t)
		if err := ref.MarkAsPaid("tx_1", now, now); err != nil {
			t.Fatalf("MarkAsPaid: %v", err)
		}

		if err := ref.Expire(tomorrow.Add(time.Hour)); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("got error %v, want ErrInvalidTransition", err)
		}
	})
}
