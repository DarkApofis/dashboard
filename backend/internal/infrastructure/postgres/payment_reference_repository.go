package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/business"
	"paymentlinks/internal/domain/paymentref"
)

// PaymentReferenceRepository is the PostgreSQL implementation of the payment
// reference port.
type PaymentReferenceRepository struct {
	base
}

var _ apppaymentref.Repository = (*PaymentReferenceRepository)(nil)

// NewPaymentReferenceRepository builds the repository over the given pool.
func NewPaymentReferenceRepository(pool *pgxpool.Pool) *PaymentReferenceRepository {
	return &PaymentReferenceRepository{base: base{pool: pool}}
}

// effectiveStatusExpr mirrors PaymentReference.EffectiveStatus in SQL: a pending
// row past its expiration date filters as expired even if the periodic job has not
// materialised it yet. The placeholder holding the reference instant is filled in
// by the caller, because its position depends on which filters are present.
const effectiveStatusExpr = `CASE WHEN status = 'pending' AND expires_at <= $%d THEN 'expired' ELSE status END`

// Create inserts a new reference and assigns its generated id.
func (r *PaymentReferenceRepository) Create(ctx context.Context, ref *paymentref.PaymentReference) error {
	const query = `
		INSERT INTO payment_references (
			business_id, external_reference, amount, description,
			status, expires_at, created_at, updated_at
		)
		VALUES ($1, $2, $3::numeric, $4, $5, $6, COALESCE($7, now()), COALESCE($8, now()))
		RETURNING id, created_at, updated_at`

	row := r.querier(ctx).QueryRow(ctx, query,
		ref.BusinessID,
		ref.ExternalReference,
		ref.Amount.String(),
		textOrNil(ref.Description),
		string(ref.Status),
		ref.ExpiresAt,
		timeOrNil(ref.CreatedAt),
		timeOrNil(ref.UpdatedAt),
	)

	if err := row.Scan(&ref.ID, &ref.CreatedAt, &ref.UpdatedAt); err != nil {
		switch {
		case isUniqueViolation(err, constraintPaymentReferenceExternalRef):
			return paymentref.ErrDuplicateReference.
				WithDetail("external_reference", ref.ExternalReference).
				Wrap(err)
		case isForeignKeyViolation(err, constraintPaymentReferenceBusiness):
			return business.ErrNotFound.Wrap(err)
		case isCheckViolation(err):
			return paymentref.ErrAmountNotPositive.Wrap(err)
		default:
			return fmt.Errorf("insert payment reference: %w", err)
		}
	}

	// PostgreSQL returns timestamptz in the session time zone; every timestamp in
	// the domain and on the wire is UTC.
	ref.CreatedAt = ref.CreatedAt.UTC()
	ref.UpdatedAt = ref.UpdatedAt.UTC()
	return nil
}

// GetByID reads one reference, or paymentref.ErrNotFound.
func (r *PaymentReferenceRepository) GetByID(ctx context.Context, id int64) (*paymentref.PaymentReference, error) {
	query := `SELECT ` + paymentReferenceColumns + ` FROM payment_references WHERE id = $1`
	return r.queryOne(ctx, query, id)
}

// GetByExternalReference reads one reference by the provider-facing identifier.
func (r *PaymentReferenceRepository) GetByExternalReference(ctx context.Context, externalReference string) (*paymentref.PaymentReference, error) {
	query := `SELECT ` + paymentReferenceColumns + ` FROM payment_references WHERE external_reference = $1`
	return r.queryOne(ctx, query, externalReference)
}

// LockByID reads one reference FOR UPDATE; call it inside TxManager.WithTx.
func (r *PaymentReferenceRepository) LockByID(ctx context.Context, id int64) (*paymentref.PaymentReference, error) {
	query := `SELECT ` + paymentReferenceColumns + ` FROM payment_references WHERE id = $1 FOR UPDATE`
	return r.queryOne(ctx, query, id)
}

// LockByExternalReference reads one reference FOR UPDATE by external reference.
// This is what serialises two concurrent webhook deliveries for the same
// reference: the second one blocks until the first commits.
func (r *PaymentReferenceRepository) LockByExternalReference(ctx context.Context, externalReference string) (*paymentref.PaymentReference, error) {
	query := `SELECT ` + paymentReferenceColumns + ` FROM payment_references WHERE external_reference = $1 FOR UPDATE`
	return r.queryOne(ctx, query, externalReference)
}

// List returns a filtered, paginated page ordered by creation date, newest first.
// The total is obtained with a window function to keep it to a single round trip.
func (r *PaymentReferenceRepository) List(ctx context.Context, filter apppaymentref.Filter, now time.Time) (apppaymentref.Page, error) {
	filter.Normalize()

	// Placeholders are numbered as the filters are appended. Only the parameters a
	// filter actually uses are bound: PostgreSQL rejects a statement carrying a
	// parameter it never references, because it cannot infer its type.
	filterArgs := make([]any, 0, 4)
	conditions := make([]string, 0, 3)

	if filter.BusinessID != nil {
		filterArgs = append(filterArgs, *filter.BusinessID)
		conditions = append(conditions, fmt.Sprintf("business_id = $%d", len(filterArgs)))
	}
	if filter.Status != nil {
		filterArgs = append(filterArgs, now.UTC())
		nowPlaceholder := len(filterArgs)
		filterArgs = append(filterArgs, string(*filter.Status))
		conditions = append(conditions, fmt.Sprintf("("+effectiveStatusExpr+") = $%d", nowPlaceholder, len(filterArgs)))
	}
	if filter.ExternalReference != nil {
		filterArgs = append(filterArgs, *filter.ExternalReference)
		conditions = append(conditions, fmt.Sprintf("external_reference = $%d", len(filterArgs)))
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	args := append(append([]any{}, filterArgs...), filter.Limit, filter.Offset)
	query := `SELECT COUNT(*) OVER () AS total, ` + paymentReferenceColumns +
		` FROM payment_references` + where +
		fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return apppaymentref.Page{}, fmt.Errorf("select payment references: %w", err)
	}
	defer rows.Close()

	page := apppaymentref.Page{
		Items:  make([]*paymentref.PaymentReference, 0, filter.Limit),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	for rows.Next() {
		var total int
		var row paymentReferenceRow
		targets := append([]any{&total}, row.scanTargets()...)
		if err := rows.Scan(targets...); err != nil {
			return apppaymentref.Page{}, fmt.Errorf("scan payment reference: %w", err)
		}
		ref, err := row.toDomain()
		if err != nil {
			return apppaymentref.Page{}, err
		}
		page.Total = total
		page.Items = append(page.Items, ref)
	}
	if err := rows.Err(); err != nil {
		return apppaymentref.Page{}, fmt.Errorf("iterate payment references: %w", err)
	}

	// COUNT(*) OVER () rides along with the rows, so an empty page carries no count:
	// an offset past the end would report a total of zero and a client could not tell
	// "nothing matches" from "you asked for a page that does not exist". The extra
	// query only runs on that empty page, so the common case is still one round trip.
	if len(page.Items) == 0 {
		total, err := r.countMatching(ctx, where, filterArgs)
		if err != nil {
			return apppaymentref.Page{}, err
		}
		page.Total = total
	}

	return page, nil
}

// countMatching counts every row the filter selects, ignoring pagination.
func (r *PaymentReferenceRepository) countMatching(ctx context.Context, where string, args []any) (int, error) {
	query := `SELECT COUNT(*) FROM payment_references` + where

	var total int
	if err := r.querier(ctx).QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count payment references: %w", err)
	}
	return total, nil
}

// UpdateWithStatusGuard persists the mutable state of a reference only while its
// stored status still matches expected. Comparing inside the UPDATE gives
// optimistic concurrency without a version column: if another transaction moved
// the row first, zero rows match and the caller is told to retry.
//
// Immutable fields (business_id, external_reference, amount, created_at) are
// deliberately absent from the SET clause.
func (r *PaymentReferenceRepository) UpdateWithStatusGuard(ctx context.Context, ref *paymentref.PaymentReference, expected paymentref.Status) error {
	const query = `
		UPDATE payment_references
		SET status = $1,
			description = $2,
			paid_at = $3,
			cancelled_at = $4,
			paid_transaction_id = $5,
			requires_review = $6,
			review_reason = $7,
			updated_at = COALESCE($8, now())
		WHERE id = $9 AND status = $10`

	tag, err := r.querier(ctx).Exec(ctx, query,
		string(ref.Status),
		textOrNil(ref.Description),
		utcOrNil(ref.PaidAt),
		utcOrNil(ref.CancelledAt),
		textOrNil(ref.PaidTransactionID),
		ref.RequiresReview,
		textOrNil(ref.ReviewReason),
		timeOrNil(ref.UpdatedAt),
		ref.ID,
		string(expected),
	)
	if err != nil {
		return fmt.Errorf("update payment reference: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return paymentref.ErrConcurrentUpdate.WithDetail("expected_status", string(expected))
	}
	return nil
}

// ExpireDue moves a bounded batch of overdue pending references to expired and
// reports how many it moved. SKIP LOCKED lets several workers run it at once
// without blocking each other, and the operation is idempotent by construction.
func (r *PaymentReferenceRepository) ExpireDue(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}

	const query = `
		UPDATE payment_references
		SET status = 'expired', updated_at = $1
		WHERE id IN (
			SELECT id
			FROM payment_references
			WHERE status = 'pending' AND expires_at <= $1
			ORDER BY expires_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)`

	tag, err := r.querier(ctx).Exec(ctx, query, now.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("expire due payment references: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *PaymentReferenceRepository) queryOne(ctx context.Context, query string, args ...any) (*paymentref.PaymentReference, error) {
	var row paymentReferenceRow
	err := r.querier(ctx).QueryRow(ctx, query, args...).Scan(row.scanTargets()...)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, paymentref.ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("select payment reference: %w", err)
	}
	return row.toDomain()
}
