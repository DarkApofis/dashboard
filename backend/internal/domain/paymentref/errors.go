package paymentref

import "paymentlinks/internal/domain/errs"

// Domain errors for payment references. Each one carries a stable code, and the
// transport layer turns its Kind into an HTTP status in exactly one place.
var (
	ErrNotFound           = errs.NotFound("payment_reference_not_found", "payment reference not found")
	ErrDuplicateReference = errs.Conflict("duplicate_external_reference", "a payment reference with that external_reference already exists")
	ErrInvalidTransition  = errs.Conflict("invalid_status_transition", "the payment reference cannot move to that status")
	ErrAlreadyPaid        = errs.Conflict("reference_already_paid", "the payment reference is already paid")

	ErrCannotCancelPaid    = errs.Conflict("cannot_cancel_paid_reference", "a paid payment reference cannot be cancelled")
	ErrCannotCancelExpired = errs.Conflict("cannot_cancel_expired_reference", "an expired payment reference cannot be cancelled")
	ErrCannotPayCancelled  = errs.Conflict("cannot_pay_cancelled_reference", "a cancelled payment reference cannot be paid")
	ErrNotYetExpired       = errs.Conflict("reference_not_yet_expired", "the payment reference has not reached its expiration date")

	// ErrConcurrentUpdate is raised when a status-guarded UPDATE matches no row
	// because another transaction changed the status first.
	ErrConcurrentUpdate = errs.Conflict("concurrent_update", "the payment reference was modified concurrently, retry the operation")

	ErrInvalidBusiness          = errs.Unprocessable("invalid_business_id", "business_id must be a positive number")
	ErrInvalidExternalReference = errs.Unprocessable("invalid_external_reference", "external_reference must be 1 to 64 printable characters with no whitespace")
	ErrAmountNotPositive        = errs.Unprocessable("invalid_amount", "amount must be greater than zero")
	ErrDescriptionTooLong       = errs.Unprocessable("invalid_description", "description must be at most 500 characters")
	ErrExpirationNotInFuture    = errs.Unprocessable("invalid_expires_at", "expires_at must be in the future")
	ErrInvalidTransactionID     = errs.Unprocessable("invalid_transaction_id", "transaction_id is required")

	// ErrDuplicateEvent is raised when a webhook event_id was already recorded. It
	// is an internal signal, not an API error: the webhook answers a redelivery
	// with the outcome of the original delivery.
	ErrDuplicateEvent = errs.Conflict("duplicate_webhook_event", "the webhook event was already processed")

	// ErrInvalidWebhookPayload is the only 4xx a well-behaved provider should ever
	// see from the webhook, alongside a rejected signature. Its Details name the
	// offending fields.
	ErrInvalidWebhookPayload = errs.Validation("invalid_webhook_payload", "the webhook payload is missing required fields or has invalid values")

	// ErrInvalidSignature is raised when HMAC verification fails.
	ErrInvalidSignature = errs.Unauthorized("invalid_signature", "the webhook signature is missing or does not match")
)
