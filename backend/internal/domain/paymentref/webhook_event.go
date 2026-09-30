package paymentref

import (
	"strings"
	"time"

	"paymentlinks/internal/domain/money"
)

// MaxEventIDLength bounds the provider-supplied idempotency key.
const MaxEventIDLength = 128

// Reasons a reference ends up flagged for human reconciliation.
const (
	ReviewReasonAmountMismatch         = "amount_mismatch"
	ReviewReasonPaymentForCancelled    = "payment_for_cancelled_reference"
	ReviewReasonConflictingTransaction = "conflicting_transaction"
)

// WebhookOutcome records what the platform did with a provider notification.
// Every delivery is persisted, including those that changed nothing, because the
// discarded ones are precisely the ones worth auditing.
type WebhookOutcome string

const (
	// OutcomeReceived is the provisional outcome a delivery is recorded with before
	// it is evaluated. It never survives a commit: the record and its final outcome
	// are written in the same transaction.
	OutcomeReceived WebhookOutcome = "received"
	// OutcomeApplied: the reference moved to paid.
	OutcomeApplied WebhookOutcome = "applied"
	// OutcomeDuplicate: the event was already processed, or the same payment
	// arrived again under a new event_id.
	OutcomeDuplicate WebhookOutcome = "duplicate"
	// OutcomeAmountMismatch: the notified amount differs from the expected one.
	OutcomeAmountMismatch WebhookOutcome = "amount_mismatch"
	// OutcomeReferenceNotFound: no reference matches the external reference.
	OutcomeReferenceNotFound WebhookOutcome = "reference_not_found"
	// OutcomeConflictCancelled: a payment arrived for a cancelled reference.
	OutcomeConflictCancelled WebhookOutcome = "conflict_cancelled"
	// OutcomeConflictOtherTransaction: an already paid reference received a
	// payment carrying a different transaction_id (possible double charge).
	OutcomeConflictOtherTransaction WebhookOutcome = "conflict_other_transaction"
	// OutcomePaidAfterExpiration: applied, but past the expiration date.
	OutcomePaidAfterExpiration WebhookOutcome = "paid_after_expiration"
)

var webhookOutcomes = map[WebhookOutcome]struct{}{
	OutcomeReceived:                 {},
	OutcomeApplied:                  {},
	OutcomeDuplicate:                {},
	OutcomeAmountMismatch:           {},
	OutcomeReferenceNotFound:        {},
	OutcomeConflictCancelled:        {},
	OutcomeConflictOtherTransaction: {},
	OutcomePaidAfterExpiration:      {},
}

// IsValid reports whether o is a known outcome.
func (o WebhookOutcome) IsValid() bool {
	_, ok := webhookOutcomes[o]
	return ok
}

func (o WebhookOutcome) String() string { return string(o) }

// Applied reports whether the outcome means the reference was moved to paid by
// this delivery.
func (o WebhookOutcome) Applied() bool {
	return o == OutcomeApplied || o == OutcomePaidAfterExpiration
}

// WebhookEvent is the durable record of one provider notification. EventID is the
// idempotency key: the unique index on it is what makes redelivery safe.
type WebhookEvent struct {
	ID                 int64
	EventID            string
	ExternalReference  string
	PaymentReferenceID *int64
	Amount             money.Money
	TransactionID      string
	PaidAt             *time.Time
	// Payload is the raw JSON body, kept verbatim so an event can be audited or
	// replayed even if the parsing rules change later.
	Payload    []byte
	Outcome    WebhookOutcome
	ReceivedAt time.Time
}

// NewWebhookEventParams is the input of NewWebhookEvent.
type NewWebhookEventParams struct {
	EventID           string
	ExternalReference string
	Amount            money.Money
	TransactionID     string
	PaidAt            time.Time
	Payload           []byte
	Now               time.Time
}

// NewWebhookEvent validates a delivery and returns the record to persist, in the
// provisional OutcomeReceived state.
//
// Everything rejected here is a malformed payload: without an event_id there is no
// idempotency key, and without an external reference there is nothing to resolve.
// Those are the only cases the provider is told to fix, with a 4xx.
func NewWebhookEvent(p NewWebhookEventParams) (*WebhookEvent, error) {
	invalid := ErrInvalidWebhookPayload

	eventID := strings.TrimSpace(p.EventID)
	if eventID == "" || len(eventID) > MaxEventIDLength {
		invalid = invalid.WithDetail("event_id", "required, at most 128 characters")
	}

	externalReference := strings.TrimSpace(p.ExternalReference)
	if !isValidExternalReference(externalReference) {
		invalid = invalid.WithDetail("external_reference", "required, at most 64 characters with no whitespace")
	}

	transactionID := strings.TrimSpace(p.TransactionID)
	if transactionID == "" {
		invalid = invalid.WithDetail("transaction_id", "required")
	}

	if !p.Amount.IsPositive() {
		invalid = invalid.WithDetail("amount", "must be greater than zero")
	}

	if p.PaidAt.IsZero() {
		invalid = invalid.WithDetail("paid_at", "required")
	}

	if len(invalid.Details) > 0 {
		return nil, invalid
	}

	paidAt := p.PaidAt.UTC()
	return &WebhookEvent{
		EventID:           eventID,
		ExternalReference: externalReference,
		Amount:            p.Amount,
		TransactionID:     transactionID,
		PaidAt:            &paidAt,
		Payload:           p.Payload,
		Outcome:           OutcomeReceived,
		ReceivedAt:        p.Now.UTC(),
	}, nil
}

// Complete stamps the final outcome and the reference the delivery resolved to.
func (e *WebhookEvent) Complete(outcome WebhookOutcome, paymentReferenceID *int64) {
	e.Outcome = outcome
	e.PaymentReferenceID = paymentReferenceID
}
