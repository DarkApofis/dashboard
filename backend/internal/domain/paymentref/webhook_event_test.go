package paymentref

import (
	"errors"
	"testing"
	"time"

	"paymentlinks/internal/domain/errs"
	"paymentlinks/internal/domain/money"
)

// errorDetails pulls the field-level details out of a domain error.
func errorDetails(t *testing.T, err error) map[string]string {
	t.Helper()

	domainErr, ok := errs.From(err)
	if !ok {
		t.Fatalf("got %v, want a domain error", err)
	}
	return domainErr.Details
}

func validEventParams() NewWebhookEventParams {
	return NewWebhookEventParams{
		EventID:           "evt_983829",
		ExternalReference: "INV-10023",
		Amount:            money.MustFromString("125000.50"),
		TransactionID:     "tx_883721",
		PaidAt:            now.Add(-time.Minute),
		Payload:           []byte(`{"event_id":"evt_983829"}`),
		Now:               now,
	}
}

func TestNewWebhookEvent(t *testing.T) {
	t.Run("a valid delivery starts as received", func(t *testing.T) {
		event, err := NewWebhookEvent(validEventParams())
		if err != nil {
			t.Fatalf("NewWebhookEvent: unexpected error %v", err)
		}

		if event.Outcome != OutcomeReceived {
			t.Errorf("outcome: got %q, want received", event.Outcome)
		}
		if event.PaymentReferenceID != nil {
			t.Error("the reference is not resolved yet at this point")
		}
		if event.PaidAt == nil || event.PaidAt.Location() != time.UTC {
			t.Errorf("paid_at must be stored in UTC, got %v", event.PaidAt)
		}
		if string(event.Payload) != `{"event_id":"evt_983829"}` {
			t.Errorf("the raw payload must be kept verbatim, got %s", event.Payload)
		}
	})

	t.Run("identifiers are trimmed", func(t *testing.T) {
		params := validEventParams()
		params.EventID = "  evt_1  "
		params.ExternalReference = " INV-1 "
		params.TransactionID = " tx_1 "

		event, err := NewWebhookEvent(params)
		if err != nil {
			t.Fatalf("NewWebhookEvent: %v", err)
		}
		if event.EventID != "evt_1" || event.ExternalReference != "INV-1" || event.TransactionID != "tx_1" {
			t.Errorf("unexpected identifiers: %+v", event)
		}
	})
}

// Everything rejected here is a malformed payload, which is one of the only two
// things the webhook answers with a 4xx.
func TestNewWebhookEventRejectsMalformedPayloads(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*NewWebhookEventParams)
		wantField string
	}{
		{"missing event id", func(p *NewWebhookEventParams) { p.EventID = "  " }, "event_id"},
		{"event id too long", func(p *NewWebhookEventParams) { p.EventID = string(make([]byte, MaxEventIDLength+1)) }, "event_id"},
		{"missing external reference", func(p *NewWebhookEventParams) { p.ExternalReference = "" }, "external_reference"},
		{"external reference with whitespace", func(p *NewWebhookEventParams) { p.ExternalReference = "INV 1" }, "external_reference"},
		{"missing transaction id", func(p *NewWebhookEventParams) { p.TransactionID = "" }, "transaction_id"},
		{"zero amount", func(p *NewWebhookEventParams) { p.Amount = money.Zero() }, "amount"},
		{"missing paid_at", func(p *NewWebhookEventParams) { p.PaidAt = time.Time{} }, "paid_at"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := validEventParams()
			tc.mutate(&params)

			_, err := NewWebhookEvent(params)
			if !errors.Is(err, ErrInvalidWebhookPayload) {
				t.Fatalf("got error %v, want ErrInvalidWebhookPayload", err)
			}

			// The details name the offending field, so the provider can fix it
			// without asking us.
			details := errorDetails(t, err)
			if _, ok := details[tc.wantField]; !ok {
				t.Errorf("details must mention %q, got %v", tc.wantField, details)
			}
		})
	}

	t.Run("several problems are reported together", func(t *testing.T) {
		params := validEventParams()
		params.EventID = ""
		params.TransactionID = ""

		_, err := NewWebhookEvent(params)
		details := errorDetails(t, err)
		if len(details) != 2 {
			t.Errorf("got %d details, want 2: %v", len(details), details)
		}
	})

	// The sentinel is shared, so a call that enriches it must not leave its details
	// behind for the next one.
	t.Run("the sentinel is never mutated", func(t *testing.T) {
		params := validEventParams()
		params.EventID = ""
		if _, err := NewWebhookEvent(params); err == nil {
			t.Fatal("expected an error")
		}

		if len(ErrInvalidWebhookPayload.Details) != 0 {
			t.Errorf("the sentinel accumulated details: %v", ErrInvalidWebhookPayload.Details)
		}
		if _, err := NewWebhookEvent(validEventParams()); err != nil {
			t.Fatalf("a valid delivery after an invalid one must pass, got %v", err)
		}
	})
}

func TestWebhookOutcome(t *testing.T) {
	applied := []WebhookOutcome{OutcomeApplied, OutcomePaidAfterExpiration}
	notApplied := []WebhookOutcome{
		OutcomeReceived,
		OutcomeDuplicate,
		OutcomeAmountMismatch,
		OutcomeReferenceNotFound,
		OutcomeConflictCancelled,
		OutcomeConflictOtherTransaction,
	}

	for _, outcome := range append(append([]WebhookOutcome{}, applied...), notApplied...) {
		if !outcome.IsValid() {
			t.Errorf("%q must be a valid outcome", outcome)
		}
	}
	if WebhookOutcome("refunded").IsValid() {
		t.Error("an unknown outcome must not validate")
	}

	for _, outcome := range applied {
		if !outcome.Applied() {
			t.Errorf("%q must report Applied", outcome)
		}
	}
	for _, outcome := range notApplied {
		if outcome.Applied() {
			t.Errorf("%q must not report Applied", outcome)
		}
	}
}

func TestWebhookEventComplete(t *testing.T) {
	event, err := NewWebhookEvent(validEventParams())
	if err != nil {
		t.Fatalf("NewWebhookEvent: %v", err)
	}

	id := int64(42)
	event.Complete(OutcomeApplied, &id)

	if event.Outcome != OutcomeApplied {
		t.Errorf("outcome: got %q, want applied", event.Outcome)
	}
	if event.PaymentReferenceID == nil || *event.PaymentReferenceID != id {
		t.Errorf("payment_reference_id: got %v, want %d", event.PaymentReferenceID, id)
	}
}
