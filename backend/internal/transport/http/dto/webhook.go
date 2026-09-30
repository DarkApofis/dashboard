package dto

import (
	"time"

	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
)

// PaymentWebhookRequest is the body of POST /webhooks/payments, as documented by
// the provider:
//
//	{
//	  "event_id": "evt_983829",
//	  "external_reference": "INV-10023",
//	  "amount": 125000.50,
//	  "transaction_id": "tx_883721",
//	  "paid_at": "2026-09-28T20:15:00Z"
//	}
//
// Unknown fields are tolerated here, unlike in the endpoints we own: the provider
// owns this contract and may add fields at any time, and rejecting a delivery over a
// field we do not care about would stop real payments from being recorded.
type PaymentWebhookRequest struct {
	EventID           string    `json:"event_id"`
	ExternalReference string    `json:"external_reference"`
	Amount            Amount    `json:"amount"`
	TransactionID     string    `json:"transaction_id"`
	PaidAt            time.Time `json:"paid_at"`
}

// ToInput maps the request onto the use case input, carrying the raw body along so
// it can be stored verbatim.
func (r PaymentWebhookRequest) ToInput(rawPayload []byte) apppaymentref.RegisterPaymentInput {
	return apppaymentref.RegisterPaymentInput{
		EventID:           r.EventID,
		ExternalReference: r.ExternalReference,
		Amount:            r.Amount.Value,
		TransactionID:     r.TransactionID,
		PaidAt:            r.PaidAt,
		RawPayload:        rawPayload,
	}
}

// PaymentWebhookResponse tells the provider what happened with the delivery.
//
// It always accompanies a 2xx when the event was recorded, including rejections, so
// the provider stops retrying; Outcome is what says whether anything was applied.
type PaymentWebhookResponse struct {
	EventID string `json:"event_id"`
	Outcome string `json:"outcome"`
	// Applied is true only when this delivery moved the reference to paid.
	Applied bool `json:"applied"`
	// Duplicate is true when the event_id had already been processed.
	Duplicate          bool   `json:"duplicate"`
	PaymentReferenceID *int64 `json:"payment_reference_id"`
	// ReferenceStatus is the effective status of the reference after the delivery.
	ReferenceStatus string `json:"reference_status,omitempty"`
	Message         string `json:"message"`
}

// outcomeMessages explains each outcome in the provider's own response, so a support
// conversation does not need access to our logs.
var outcomeMessages = map[domain.WebhookOutcome]string{
	domain.OutcomeApplied:                  "payment applied, the reference is now paid",
	domain.OutcomeDuplicate:                "event already processed, no changes applied",
	domain.OutcomeAmountMismatch:           "notified amount does not match the reference amount, recorded for review",
	domain.OutcomeReferenceNotFound:        "no payment reference matches that external_reference, recorded for review",
	domain.OutcomeConflictCancelled:        "the payment reference was cancelled, recorded for review",
	domain.OutcomeConflictOtherTransaction: "the reference was already paid by a different transaction, recorded for review",
	domain.OutcomePaidAfterExpiration:      "payment applied after the expiration date, flagged for review",
}

// NewPaymentWebhookResponse maps the use case result.
func NewPaymentWebhookResponse(result apppaymentref.RegisterPaymentResult) PaymentWebhookResponse {
	message, ok := outcomeMessages[result.Outcome]
	if !ok {
		message = "event recorded"
	}

	response := PaymentWebhookResponse{
		EventID:   result.EventID,
		Outcome:   string(result.Outcome),
		Applied:   result.Applied,
		Duplicate: result.Duplicate,
		Message:   message,
	}

	if result.Reference != nil {
		response.PaymentReferenceID = &result.Reference.ID
		response.ReferenceStatus = string(result.Reference.EffectiveStatus(result.ObservedAt))
	}
	return response
}
