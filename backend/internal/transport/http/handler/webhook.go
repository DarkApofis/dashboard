package handler

import (
	"net/http"

	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/respond"
)

// Webhook holds the provider notification endpoints.
type Webhook struct {
	registerPayment *apppaymentref.RegisterPayment
}

// NewWebhook wires the handler with its use case.
func NewWebhook(registerPayment *apppaymentref.RegisterPayment) *Webhook {
	return &Webhook{registerPayment: registerPayment}
}

// RegisterPayment handles POST /webhooks/payments.
//
// The status code answers one question only: did we durably record the delivery?
//
//   - 200 when we did, including when the payment was rejected on business grounds
//     (unknown reference, amount mismatch, cancelled reference, conflicting
//     transaction). The provider retries on anything that is not a 2xx, and
//     retrying will produce the exact same rejection forever: the problem needs a
//     human, not another delivery. The rejection is already persisted in
//     webhook_events and the reference is flagged for review, so nothing is lost by
//     acknowledging. The outcome field in the body is what says what happened.
//   - 400 only when the payload is unusable, and 401 when the signature does not
//     verify. Those are the only cases a retry could plausibly fix, and the only
//     ones that are the provider's to fix.
//   - 500 when we failed to record it, which is exactly when we want the retry.
func (h *Webhook) RegisterPayment(w http.ResponseWriter, r *http.Request) {
	// The raw body is read first because it is stored verbatim for auditing, and
	// because the provider's contract allows fields we do not model.
	var request dto.PaymentWebhookRequest
	raw, err := readJSON(r, &request)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	result, err := h.registerPayment.Execute(r.Context(), request.ToInput(raw))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, dto.NewPaymentWebhookResponse(result))
}
