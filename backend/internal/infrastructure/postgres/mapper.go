package postgres

import (
	"fmt"
	"time"

	"paymentlinks/internal/domain/money"
	"paymentlinks/internal/domain/paymentref"
)

// Amounts cross the driver boundary as text.
//
// NUMERIC has no native Go counterpart in pgx, and decoding it into float64 would
// defeat the whole point of using NUMERIC. Selecting `amount::text` yields the
// exact decimal literal ("125000.50") and binding a string with an explicit
// ::numeric cast on the way in keeps the round trip exact, with no extra
// dependency and with the conversion confined to this file.
const paymentReferenceColumns = `
	id,
	business_id,
	external_reference,
	amount::text,
	COALESCE(description, ''),
	status,
	expires_at,
	created_at,
	updated_at,
	paid_at,
	cancelled_at,
	COALESCE(paid_transaction_id, ''),
	requires_review,
	COALESCE(review_reason, '')`

// paymentReferenceRow is the flat shape returned by the queries above.
type paymentReferenceRow struct {
	id                int64
	businessID        int64
	externalReference string
	amount            string
	description       string
	status            string
	expiresAt         time.Time
	createdAt         time.Time
	updatedAt         time.Time
	paidAt            *time.Time
	cancelledAt       *time.Time
	paidTransactionID string
	requiresReview    bool
	reviewReason      string
}

// scanTargets lists the destinations in column order.
func (row *paymentReferenceRow) scanTargets() []any {
	return []any{
		&row.id,
		&row.businessID,
		&row.externalReference,
		&row.amount,
		&row.description,
		&row.status,
		&row.expiresAt,
		&row.createdAt,
		&row.updatedAt,
		&row.paidAt,
		&row.cancelledAt,
		&row.paidTransactionID,
		&row.requiresReview,
		&row.reviewReason,
	}
}

// toDomain rebuilds the entity. A row that cannot be mapped is a data integrity
// problem, not a validation error, so it surfaces as a plain wrapped error.
func (row *paymentReferenceRow) toDomain() (*paymentref.PaymentReference, error) {
	amount, err := money.FromString(row.amount)
	if err != nil {
		return nil, fmt.Errorf("payment_references.id=%d has an unusable amount %q: %w", row.id, row.amount, err)
	}

	status, err := paymentref.ParseStatus(row.status)
	if err != nil {
		return nil, fmt.Errorf("payment_references.id=%d has an unknown status %q: %w", row.id, row.status, err)
	}

	return &paymentref.PaymentReference{
		ID:                row.id,
		BusinessID:        row.businessID,
		ExternalReference: row.externalReference,
		Amount:            amount,
		Description:       row.description,
		Status:            status,
		ExpiresAt:         row.expiresAt.UTC(),
		CreatedAt:         row.createdAt.UTC(),
		UpdatedAt:         row.updatedAt.UTC(),
		PaidAt:            utcOrNil(row.paidAt),
		CancelledAt:       utcOrNil(row.cancelledAt),
		PaidTransactionID: row.paidTransactionID,
		RequiresReview:    row.requiresReview,
		ReviewReason:      row.reviewReason,
	}, nil
}

// timeOrNil maps the zero time to a SQL NULL so the column default can apply.
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// utcOrNil normalises an optional timestamp to UTC.
func utcOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// textOrNil maps the empty string to a SQL NULL, keeping "absent" and "empty"
// indistinguishable in the database and empty in Go.
func textOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
