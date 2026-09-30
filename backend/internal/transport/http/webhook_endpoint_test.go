package httpapi_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paymentlinks/internal/transport/http/dto"
)

const webhookPath = "/webhooks/payments"

func webhookBody(eventID, externalReference, amount, transactionID string) string {
	return `{"event_id":"` + eventID +
		`","external_reference":"` + externalReference +
		`","amount":` + amount +
		`,"transaction_id":"` + transactionID +
		`","paid_at":"2026-09-28T20:15:00Z"}`
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func decodeWebhook(t *testing.T, recorder *httptest.ResponseRecorder) dto.PaymentWebhookResponse {
	t.Helper()

	var body dto.PaymentWebhookResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode webhook response: %v (body %s)", err, recorder.Body.String())
	}
	return body
}

// seedPaidableReference creates a pending reference of 125000.50 through the API.
func (a *api) seedPaidableReference(t *testing.T, externalReference string) dto.PaymentReferenceResponse {
	t.Helper()

	recorder := a.do(t, http.MethodPost, "/payment-references", a.createBody(externalReference))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("seed reference: got %d (body %s)", recorder.Code, recorder.Body.String())
	}
	return decodeReference(t, recorder)
}

func TestWebhookAppliesPayment(t *testing.T) {
	a := newAPI(t)
	created := a.seedPaidableReference(t, "INV-10023")

	recorder := a.do(t, http.MethodPost, webhookPath, webhookBody("evt_983829", "INV-10023", "125000.50", "tx_883721"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	body := decodeWebhook(t, recorder)
	if body.Outcome != "applied" || !body.Applied || body.Duplicate {
		t.Errorf("unexpected response: %+v", body)
	}
	if body.ReferenceStatus != "paid" {
		t.Errorf("reference_status: got %q, want paid", body.ReferenceStatus)
	}

	// And the reference really is paid when read back through the API.
	reloaded := decodeReference(t, a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), ""))
	if reloaded.Status != "paid" || reloaded.TransactionID != "tx_883721" || reloaded.PaidAt == nil {
		t.Errorf("unexpected reference: %+v", reloaded)
	}
}

// A redelivery is the normal case, not an error: the provider retries whenever it
// does not get a response.
func TestWebhookRedeliveryAnswers200WithoutReapplying(t *testing.T) {
	a := newAPI(t)
	a.seedPaidableReference(t, "INV-10023")
	body := webhookBody("evt_983829", "INV-10023", "125000.50", "tx_883721")

	if code := a.do(t, http.MethodPost, webhookPath, body).Code; code != http.StatusOK {
		t.Fatalf("first delivery: got %d", code)
	}

	recorder := a.do(t, http.MethodPost, webhookPath, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("redelivery: got %d, want 200", recorder.Code)
	}

	replayed := decodeWebhook(t, recorder)
	if !replayed.Duplicate {
		t.Error("the redelivery must be reported as a duplicate")
	}
	if replayed.Applied {
		t.Error("the redelivery must not apply anything again")
	}
	if replayed.Outcome != "applied" {
		t.Errorf("outcome: got %q, want the original outcome applied", replayed.Outcome)
	}
	if a.events.Insertions() != 1 {
		t.Errorf("recorded %d events, want 1", a.events.Insertions())
	}
}

// Every business rejection is a 200: the delivery was recorded, and retrying would
// produce the same rejection forever.
func TestWebhookBusinessRejectionsAnswer200(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(t *testing.T, a *api)
		body        func(a *api) string
		wantOutcome string
	}{
		{
			name: "amount mismatch",
			setup: func(t *testing.T, a *api) {
				a.seedPaidableReference(t, "INV-1")
			},
			body:        func(*api) string { return webhookBody("evt_1", "INV-1", "999.99", "tx_1") },
			wantOutcome: "amount_mismatch",
		},
		{
			name: "cancelled reference",
			setup: func(t *testing.T, a *api) {
				created := a.seedPaidableReference(t, "INV-1")
				if code := a.do(t, http.MethodPost, "/payment-references/"+itoa(created.ID)+"/cancel", "").Code; code != http.StatusOK {
					t.Fatalf("cancel: got %d", code)
				}
			},
			body:        func(*api) string { return webhookBody("evt_1", "INV-1", "125000.50", "tx_1") },
			wantOutcome: "conflict_cancelled",
		},
		{
			name: "already paid by another transaction",
			setup: func(t *testing.T, a *api) {
				a.seedPaidableReference(t, "INV-1")
				if code := a.do(t, http.MethodPost, webhookPath, webhookBody("evt_first", "INV-1", "125000.50", "tx_first")).Code; code != http.StatusOK {
					t.Fatalf("first payment: got %d", code)
				}
			},
			body:        func(*api) string { return webhookBody("evt_1", "INV-1", "125000.50", "tx_second") },
			wantOutcome: "conflict_other_transaction",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newAPI(t)
			tc.setup(t, a)

			recorder := a.do(t, http.MethodPost, webhookPath, tc.body(a))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}

			body := decodeWebhook(t, recorder)
			if body.Outcome != tc.wantOutcome {
				t.Errorf("outcome: got %q, want %q", body.Outcome, tc.wantOutcome)
			}
			if body.Applied {
				t.Error("a rejection must not report applied")
			}
			if body.Message == "" {
				t.Error("the response must explain what happened")
			}
		})
	}
}

func TestWebhookPaysExpiredReferenceAndFlagsIt(t *testing.T) {
	a := newAPI(t)
	created := a.seedPaidableReference(t, "INV-1")
	a.clock.Advance(72 * time.Hour)

	recorder := a.do(t, http.MethodPost, webhookPath, webhookBody("evt_1", "INV-1", "125000.50", "tx_1"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", recorder.Code)
	}

	body := decodeWebhook(t, recorder)
	if body.Outcome != "paid_after_expiration" || !body.Applied {
		t.Errorf("unexpected response: %+v", body)
	}

	reloaded := decodeReference(t, a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), ""))
	if reloaded.Status != "paid" || !reloaded.RequiresReview {
		t.Errorf("unexpected reference: %+v", reloaded)
	}
	if reloaded.ReviewReason != "paid_after_expiration" {
		t.Errorf("review_reason: got %q", reloaded.ReviewReason)
	}
}

// 4xx is reserved for what the provider can actually fix.
func TestWebhookRejectsMalformedPayloads(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int
	}{
		{"not JSON at all", `{`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
		{"missing event_id", `{"external_reference":"INV-1","amount":10.00,"transaction_id":"tx_1","paid_at":"2026-09-28T20:15:00Z"}`, http.StatusBadRequest},
		{"missing transaction_id", `{"event_id":"evt_1","external_reference":"INV-1","amount":10.00,"paid_at":"2026-09-28T20:15:00Z"}`, http.StatusBadRequest},
		{"missing paid_at", `{"event_id":"evt_1","external_reference":"INV-1","amount":10.00,"transaction_id":"tx_1"}`, http.StatusBadRequest},
		{"amount with three decimals", `{"event_id":"evt_1","external_reference":"INV-1","amount":10.005,"transaction_id":"tx_1","paid_at":"2026-09-28T20:15:00Z"}`, http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newAPI(t)

			recorder := a.do(t, http.MethodPost, webhookPath, tc.body)
			if recorder.Code != tc.wantCode {
				t.Fatalf("status: got %d, want %d (body %s)", recorder.Code, tc.wantCode, recorder.Body.String())
			}
			if a.events.Insertions() != 0 {
				t.Errorf("an unusable payload must not be recorded, got %d events", a.events.Insertions())
			}
		})
	}
}

// The provider may add fields at any time; that must not stop payments from being
// recorded.
func TestWebhookToleratesUnknownFields(t *testing.T) {
	a := newAPI(t)
	a.seedPaidableReference(t, "INV-1")

	body := `{"event_id":"evt_1","external_reference":"INV-1","amount":125000.50,"transaction_id":"tx_1",` +
		`"paid_at":"2026-09-28T20:15:00Z","payment_method":"pse","provider_fee":1500.00}`

	recorder := a.do(t, http.MethodPost, webhookPath, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if body := decodeWebhook(t, recorder); body.Outcome != "applied" {
		t.Errorf("outcome: got %q, want applied", body.Outcome)
	}
}

func TestWebhookSignature(t *testing.T) {
	const secret = "s3cr3t"

	t.Run("a valid signature is accepted", func(t *testing.T) {
		a := newAPIWithSecret(t, secret)
		a.seedPaidableReference(t, "INV-1")
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")

		recorder := a.doSigned(t, http.MethodPost, webhookPath, body, sign(secret, body))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("a bare hex digest is accepted too", func(t *testing.T) {
		a := newAPIWithSecret(t, secret)
		a.seedPaidableReference(t, "INV-1")
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")
		bare := sign(secret, body)[len("sha256="):]

		if code := a.doSigned(t, http.MethodPost, webhookPath, body, bare).Code; code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", code)
		}
	})

	t.Run("a missing signature is rejected", func(t *testing.T) {
		a := newAPIWithSecret(t, secret)
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")

		recorder := a.doSigned(t, http.MethodPost, webhookPath, body, "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want 401", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "invalid_signature" {
			t.Errorf("error code: got %q", code)
		}
		if a.events.Insertions() != 0 {
			t.Error("an unauthenticated delivery must not be recorded")
		}
	})

	t.Run("a signature for a different body is rejected", func(t *testing.T) {
		a := newAPIWithSecret(t, secret)
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")
		tampered := webhookBody("evt_1", "INV-1", "1.00", "tx_1")

		// The signature is valid, but for the original amount: this is exactly the
		// attack the signature exists to stop.
		recorder := a.doSigned(t, http.MethodPost, webhookPath, tampered, sign(secret, body))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want 401", recorder.Code)
		}
	})

	t.Run("a signature with the wrong secret is rejected", func(t *testing.T) {
		a := newAPIWithSecret(t, secret)
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")

		if code := a.doSigned(t, http.MethodPost, webhookPath, body, sign("wrong", body)).Code; code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want 401", code)
		}
	})

	t.Run("verification is skipped when no secret is configured", func(t *testing.T) {
		a := newAPI(t) // no secret
		a.seedPaidableReference(t, "INV-1")
		body := webhookBody("evt_1", "INV-1", "125000.50", "tx_1")

		if code := a.doSigned(t, http.MethodPost, webhookPath, body, "").Code; code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", code)
		}
	})
}

// The rule the spec states outright: a paid reference cannot be cancelled. It can
// only be checked end to end now that a reference can actually be paid through the
// API, so it lives next to the webhook tests.
func TestCancelPaidReferenceIsRejected(t *testing.T) {
	a := newAPI(t)
	created := a.seedPaidableReference(t, "INV-10023")

	paid := a.do(t, http.MethodPost, webhookPath, webhookBody("evt_1", "INV-10023", "125000.50", "tx_883721"))
	if paid.Code != http.StatusOK {
		t.Fatalf("pay the reference: got %d (body %s)", paid.Code, paid.Body.String())
	}

	recorder := a.do(t, http.MethodPost, "/payment-references/"+itoa(created.ID)+"/cancel", "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409 (body %s)", recorder.Code, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "cannot_cancel_paid_reference" {
		t.Errorf("error code: got %q, want cannot_cancel_paid_reference", code)
	}

	// The rejection must not have disturbed anything.
	reloaded := decodeReference(t, a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), ""))
	if reloaded.Status != "paid" {
		t.Errorf("status: got %q, want paid", reloaded.Status)
	}
	if reloaded.CancelledAt != nil {
		t.Error("a rejected cancellation must not set cancelled_at")
	}

	// And retrying is still a 409, not an accidental success.
	if code := a.do(t, http.MethodPost, "/payment-references/"+itoa(created.ID)+"/cancel", "").Code; code != http.StatusConflict {
		t.Errorf("retry: got %d, want 409", code)
	}
}

// The one rejection that answers 4xx: it is also the only one a redelivery can fix.
func TestWebhookUnknownReferenceAnswers404AndRecoversLater(t *testing.T) {
	a := newAPI(t)
	body := webhookBody("evt_race", "INV-LATE", "125000.50", "tx_883721")

	recorder := a.do(t, http.MethodPost, webhookPath, body)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404 (body %s)", recorder.Code, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "payment_reference_not_found" {
		t.Errorf("error code: got %q", code)
	}
	if a.events.Insertions() != 1 {
		t.Errorf("the delivery must be recorded anyway, got %d events", a.events.Insertions())
	}

	// The business registers the charge, and the provider retries.
	created := a.seedPaidableReference(t, "INV-LATE")

	retry := a.do(t, http.MethodPost, webhookPath, body)
	if retry.Code != http.StatusOK {
		t.Fatalf("retry: got %d, want 200 (body %s)", retry.Code, retry.Body.String())
	}
	if replayed := decodeWebhook(t, retry); replayed.Outcome != "applied" || !replayed.Applied {
		t.Errorf("unexpected response: %+v", replayed)
	}

	reloaded := decodeReference(t, a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), ""))
	if reloaded.Status != "paid" {
		t.Errorf("status: got %q, want paid", reloaded.Status)
	}
}
