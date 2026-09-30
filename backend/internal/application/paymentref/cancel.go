package paymentref

import (
	"context"

	domain "paymentlinks/internal/domain/paymentref"
)

// CancelReference cancels a pending payment reference.
type CancelReference struct {
	tx         TxManager
	references Repository
	clock      Clock
}

// NewCancelReference wires the use case with its ports.
func NewCancelReference(tx TxManager, references Repository, clock Clock) *CancelReference {
	return &CancelReference{tx: tx, references: references, clock: clock}
}

// Execute cancels the reference inside a transaction.
//
// The row is read with SELECT ... FOR UPDATE so that a webhook confirming payment
// for the same reference cannot slip between the read and the write: one of the two
// waits for the other to commit and then sees the committed state. The guarded
// UPDATE is a second line of defence for the same race.
//
// The decision of whether cancelling is allowed belongs to the entity: this use
// case only orchestrates locking, persistence and the commit boundary.
func (uc *CancelReference) Execute(ctx context.Context, id int64) (Result, error) {
	var (
		result Result
		// cancelErr carries a domain rejection out of the transaction without
		// rolling it back, which matters for the expired case below.
		cancelErr error
	)

	txErr := uc.tx.WithTx(ctx, func(ctx context.Context) error {
		now := uc.clock.Now()

		reference, err := uc.references.LockByID(ctx, id)
		if err != nil {
			return err
		}

		// A pending reference whose date has passed is already expired, even if the
		// periodic job has not written it yet. Since the row is locked here anyway,
		// the expiration is materialised and committed, and the cancellation is
		// then rejected: the caller gets the accurate 409 and the stored state
		// converges instead of drifting until the job runs.
		if reference.IsExpiredAt(now) {
			if err := reference.Expire(now); err != nil {
				return err
			}
			if err := uc.references.UpdateWithStatusGuard(ctx, reference, domain.StatusPending); err != nil {
				return err
			}
			result = Result{Reference: reference, ObservedAt: now}
			cancelErr = domain.ErrCannotCancelExpired
			return nil
		}

		previousStatus := reference.Status
		if err := reference.Cancel(now); err != nil {
			cancelErr = err
			result = Result{Reference: reference, ObservedAt: now}
			return nil
		}

		// Cancel is idempotent: an already cancelled reference comes back unchanged,
		// so there is nothing to persist and the response is still a success.
		if reference.Status != previousStatus {
			if err := uc.references.UpdateWithStatusGuard(ctx, reference, previousStatus); err != nil {
				return err
			}
		}

		result = Result{Reference: reference, ObservedAt: now}
		return nil
	})

	if txErr != nil {
		return Result{}, txErr
	}
	if cancelErr != nil {
		return Result{}, cancelErr
	}
	return result, nil
}
