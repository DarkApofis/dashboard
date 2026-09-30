package paymentref

import (
	"context"
	"errors"
	"time"

	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
)

// RegisterPaymentInput is one notification from the financial provider.
type RegisterPaymentInput struct {
	EventID           string
	ExternalReference string
	Amount            money.Money
	TransactionID     string
	PaidAt            time.Time
	// RawPayload is the body as received, stored verbatim for auditing.
	RawPayload []byte
}

// RegisterPaymentResult describes what the platform did with the delivery.
type RegisterPaymentResult struct {
	EventID   string
	Outcome   domain.WebhookOutcome
	Applied   bool
	Duplicate bool
	// Reference is nil when no reference matched the external reference.
	Reference  *domain.PaymentReference
	ObservedAt time.Time
}

// RegisterPayment applies a payment notified by the financial provider.
type RegisterPayment struct {
	tx         TxManager
	references Repository
	events     WebhookEventRepository
	clock      Clock
}

// NewRegisterPayment wires the use case with its ports.
func NewRegisterPayment(tx TxManager, references Repository, events WebhookEventRepository, clock Clock) *RegisterPayment {
	return &RegisterPayment{tx: tx, references: references, events: events, clock: clock}
}

// Execute records and applies one provider notification.
//
// Everything happens in a single transaction, in this order:
//
//  1. Insert the event. The UNIQUE index on event_id, not a prior SELECT, is what
//     decides whether the delivery is new. A check-then-insert would leave a window
//     in which two concurrent deliveries both pass the check.
//  2. Lock the reference with SELECT ... FOR UPDATE, so a concurrent cancellation
//     cannot interleave between the read and the write.
//  3. Evaluate the business rules and, when they allow it, move the reference to
//     paid with the transaction id and the payment timestamp.
//  4. Stamp the final outcome on the event.
//
// Either all of that commits or none of it does: an event can never be recorded as
// applied without the reference having moved, and vice versa.
//
// Business rejections still return a nil error. The transport layer answers 2xx for
// them, and the reason is in the comment on the rejection branches below.
func (uc *RegisterPayment) Execute(ctx context.Context, in RegisterPaymentInput) (RegisterPaymentResult, error) {
	now := uc.clock.Now()

	// A malformed payload is rejected before anything is recorded: without an
	// event_id there is no idempotency key to record it under.
	event, err := domain.NewWebhookEvent(domain.NewWebhookEventParams{
		EventID:           in.EventID,
		ExternalReference: in.ExternalReference,
		Amount:            in.Amount,
		TransactionID:     in.TransactionID,
		PaidAt:            in.PaidAt,
		Payload:           in.RawPayload,
		Now:               now,
	})
	if err != nil {
		return RegisterPaymentResult{}, err
	}

	var result RegisterPaymentResult

	txErr := uc.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := uc.events.Insert(ctx, event); err != nil {
			if !errors.Is(err, domain.ErrDuplicateEvent) {
				return err
			}

			stored, err := uc.events.GetByEventID(ctx, event.EventID)
			if err != nil {
				return err
			}

			// Every outcome is final except one. A delivery that found no reference
			// can succeed later, because the reference may have been created in
			// between: the provider confirms the payment before the business
			// registers the charge more often than one would like. Replaying the
			// original "not found" would burn the event_id and lose the payment for
			// good, so that single case is re-evaluated against the current state.
			//
			// The event row is reused rather than inserted again, so there is still
			// exactly one row per event_id and its outcome converges.
			if stored.Outcome != domain.OutcomeReferenceNotFound {
				// The provider redelivers when it did not get our response. Anything
				// else was already decided, so nothing is applied twice: the original
				// outcome is replayed. The repository uses ON CONFLICT DO NOTHING
				// precisely so that this path leaves the transaction usable for the
				// read above.
				replayed, err := uc.replay(ctx, stored, now)
				if err != nil {
					return err
				}
				result = replayed
				return nil
			}
			event.ID = stored.ID
		}

		reference, err := uc.references.LockByExternalReference(ctx, event.ExternalReference)
		if err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
			// No reference matches. Recording the delivery is all we can do, and it
			// is worth doing: it is the evidence for whoever reconciles later.
			return uc.complete(ctx, event, nil, domain.OutcomeReferenceNotFound, false, now, &result)
		}

		// Captured before apply mutates the entity: this is the status the guarded
		// UPDATE will compare against, so the write is refused if anything slipped in
		// between despite the lock.
		lockedStatus := reference.Status

		outcome, applied, mutated := uc.apply(reference, event, now)

		if mutated {
			if err := uc.references.UpdateWithStatusGuard(ctx, reference, lockedStatus); err != nil {
				return err
			}
		}

		return uc.complete(ctx, event, reference, outcome, applied, now, &result)
	})
	if txErr != nil {
		return RegisterPaymentResult{}, txErr
	}

	// Recorded, and then reported as 404 on purpose.
	//
	// Every other rejection is deterministic: retrying it produces the same answer
	// forever, so it gets a 2xx to stop the provider. This one is not. The reference
	// can appear a second later, so it is the only rejection a redelivery can fix,
	// and answering 2xx would tell the provider to stop retrying a payment we could
	// still apply. The delivery stays recorded either way.
	if result.Outcome == domain.OutcomeReferenceNotFound {
		return result, domain.ErrNotFound.WithMessage(
			"no payment reference matches external_reference %q; the delivery was recorded and will be applied if it is redelivered once the reference exists",
			event.ExternalReference,
		)
	}

	return result, nil
}

// apply evaluates the delivery against the locked reference and mutates it when the
// rules allow. It returns the outcome to record, whether the reference was moved to
// paid, and whether it needs to be persisted.
//
// The order of the branches is deliberate: state conflicts are decided before the
// amount is compared, because a payment for a cancelled reference is a conflict
// regardless of how much money it carries.
func (uc *RegisterPayment) apply(
	reference *domain.PaymentReference,
	event *domain.WebhookEvent,
	now time.Time,
) (domain.WebhookOutcome, bool, bool) {
	switch {
	case reference.EffectiveStatus(now) == domain.StatusCancelled:
		// Cancelling is an explicit business decision. Applying the payment anyway
		// would silently contradict it, so the reference is left cancelled and
		// flagged instead: a human decides whether to refund.
		reference.FlagForReview(domain.ReviewReasonPaymentForCancelled)
		return domain.OutcomeConflictCancelled, false, true

	case reference.Status == domain.StatusPaid:
		if reference.PaidTransactionID == event.TransactionID {
			// Same payment arriving under a new event_id: nothing to change.
			return domain.OutcomeDuplicate, false, false
		}
		// A second, different transaction against a paid reference is a possible
		// double charge. The first payment is not overwritten.
		reference.FlagForReview(domain.ReviewReasonConflictingTransaction)
		return domain.OutcomeConflictOtherTransaction, false, true

	case !reference.Amount.Equal(event.Amount):
		// Exact decimal comparison, never float: 125000.50 has no exact binary
		// representation, so a float compare would be a coin toss. A mismatch is
		// never a partial payment here, since partial payments are out of scope.
		reference.FlagForReview(domain.ReviewReasonAmountMismatch)
		return domain.OutcomeAmountMismatch, false, true

	default:
		// A past-due reference is still marked paid: the money did arrive, and
		// refusing it would leave the business holding a payment with no record of
		// it. MarkAsPaid flags it for review in that case.
		paidAfterExpiration := reference.IsExpiredAt(now) || reference.Status == domain.StatusExpired

		if err := reference.MarkAsPaid(event.TransactionID, *event.PaidAt, now); err != nil {
			// Unreachable: the branches above already excluded every state
			// MarkAsPaid refuses. Recorded as a conflict rather than silently
			// dropped if the state machine ever grows a case this switch misses.
			reference.FlagForReview(domain.ReviewReasonConflictingTransaction)
			return domain.OutcomeConflictOtherTransaction, false, true
		}

		if paidAfterExpiration {
			return domain.OutcomePaidAfterExpiration, true, true
		}
		return domain.OutcomeApplied, true, true
	}
}

// replay returns the outcome of a delivery that was already decided.
func (uc *RegisterPayment) replay(
	ctx context.Context,
	stored *domain.WebhookEvent,
	now time.Time,
) (RegisterPaymentResult, error) {
	result := RegisterPaymentResult{
		EventID:    stored.EventID,
		Outcome:    stored.Outcome,
		Applied:    false, // nothing was applied by this delivery
		Duplicate:  true,
		ObservedAt: now,
	}

	if stored.PaymentReferenceID != nil {
		reference, err := uc.references.GetByID(ctx, *stored.PaymentReferenceID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return RegisterPaymentResult{}, err
		}
		result.Reference = reference
	}
	return result, nil
}

// complete stamps the outcome on the event and builds the result.
func (uc *RegisterPayment) complete(
	ctx context.Context,
	event *domain.WebhookEvent,
	reference *domain.PaymentReference,
	outcome domain.WebhookOutcome,
	applied bool,
	now time.Time,
	result *RegisterPaymentResult,
) error {
	var referenceID *int64
	if reference != nil {
		referenceID = &reference.ID
	}

	event.Complete(outcome, referenceID)
	if err := uc.events.Finalize(ctx, event); err != nil {
		return err
	}

	*result = RegisterPaymentResult{
		EventID:    event.EventID,
		Outcome:    outcome,
		Applied:    applied,
		Reference:  reference,
		ObservedAt: now,
	}
	return nil
}
