package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/money"
	"paymentlinks/internal/domain/paymentref"
)

// WebhookEventRepository is the PostgreSQL implementation of the webhook event
// port. The unique index on event_id is the idempotency mechanism: Insert fails
// with paymentref.ErrDuplicateEvent when the provider redelivers an event, which
// is cheaper and safer than any application-level lock.
type WebhookEventRepository struct {
	base
}

var _ apppaymentref.WebhookEventRepository = (*WebhookEventRepository)(nil)

// NewWebhookEventRepository builds the repository over the given pool.
func NewWebhookEventRepository(pool *pgxpool.Pool) *WebhookEventRepository {
	return &WebhookEventRepository{base: base{pool: pool}}
}

const webhookEventColumns = `
	id,
	event_id,
	external_reference,
	payment_reference_id,
	COALESCE(amount::text, ''),
	COALESCE(transaction_id, ''),
	paid_at,
	payload,
	outcome,
	received_at`

// Insert records one provider notification.
//
// ON CONFLICT DO NOTHING rather than letting the unique violation surface: a raw
// 23505 puts the PostgreSQL transaction in an aborted state, in which every
// subsequent statement fails. The webhook use case needs to read the original
// outcome after a redelivery, in the same transaction, so the conflict has to be a
// missing RETURNING row instead of an error. The uniqueness guarantee is identical;
// only the failure mode changes.
func (r *WebhookEventRepository) Insert(ctx context.Context, event *paymentref.WebhookEvent) error {
	const query = `
		INSERT INTO webhook_events (
			event_id, external_reference, payment_reference_id, amount,
			transaction_id, paid_at, payload, outcome, received_at
		)
		VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, COALESCE($9, now()))
		ON CONFLICT (event_id) DO NOTHING
		RETURNING id, received_at`

	payload := event.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}

	row := r.querier(ctx).QueryRow(ctx, query,
		event.EventID,
		event.ExternalReference,
		event.PaymentReferenceID,
		event.Amount.String(),
		textOrNil(event.TransactionID),
		utcOrNil(event.PaidAt),
		payload,
		string(event.Outcome),
		timeOrNil(event.ReceivedAt),
	)

	if err := row.Scan(&event.ID, &event.ReceivedAt); err != nil {
		// No row returned means the ON CONFLICT clause swallowed the insert: this
		// event_id was already recorded.
		if errors.Is(err, pgx.ErrNoRows) || isUniqueViolation(err, constraintWebhookEventEventID) {
			return paymentref.ErrDuplicateEvent.WithDetail("event_id", event.EventID).Wrap(err)
		}
		return fmt.Errorf("insert webhook event: %w", err)
	}
	event.ReceivedAt = event.ReceivedAt.UTC()
	return nil
}

// Finalize stamps the outcome and the resolved reference on an already inserted
// event. Only these two columns are mutable: the payload and the identifiers are
// what the provider sent and must stay as received.
func (r *WebhookEventRepository) Finalize(ctx context.Context, event *paymentref.WebhookEvent) error {
	const query = `
		UPDATE webhook_events
		SET outcome = $1, payment_reference_id = $2
		WHERE id = $3`

	tag, err := r.querier(ctx).Exec(ctx, query, string(event.Outcome), event.PaymentReferenceID, event.ID)
	if err != nil {
		return fmt.Errorf("finalize webhook event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("finalize webhook event: no row with id %d", event.ID)
	}
	return nil
}

// GetByEventID reads a previously recorded event, or paymentref.ErrNotFound. It
// lets the webhook use case answer a redelivery with the original outcome instead
// of a bare acknowledgement.
func (r *WebhookEventRepository) GetByEventID(ctx context.Context, eventID string) (*paymentref.WebhookEvent, error) {
	query := `SELECT ` + webhookEventColumns + ` FROM webhook_events WHERE event_id = $1`

	var (
		event     paymentref.WebhookEvent
		rawAmount string
		outcome   string
	)

	err := r.querier(ctx).QueryRow(ctx, query, eventID).Scan(
		&event.ID,
		&event.EventID,
		&event.ExternalReference,
		&event.PaymentReferenceID,
		&rawAmount,
		&event.TransactionID,
		&event.PaidAt,
		&event.Payload,
		&outcome,
		&event.ReceivedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, paymentref.ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("select webhook event: %w", err)
	}

	if rawAmount != "" {
		amount, err := money.FromString(rawAmount)
		if err != nil {
			return nil, fmt.Errorf("webhook_events.id=%d has an unusable amount %q: %w", event.ID, rawAmount, err)
		}
		event.Amount = amount
	}

	event.Outcome = paymentref.WebhookOutcome(outcome)
	event.PaidAt = utcOrNil(event.PaidAt)
	event.ReceivedAt = event.ReceivedAt.UTC()
	return &event, nil
}
