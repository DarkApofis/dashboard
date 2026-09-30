package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appbusiness "paymentlinks/internal/application/business"
	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/business"
)

// BusinessRepository is the PostgreSQL implementation of the business port.
type BusinessRepository struct {
	base
}

var (
	_ appbusiness.Repository       = (*BusinessRepository)(nil)
	_ apppaymentref.BusinessFinder = (*BusinessRepository)(nil)
)

// NewBusinessRepository builds the repository over the given pool.
func NewBusinessRepository(pool *pgxpool.Pool) *BusinessRepository {
	return &BusinessRepository{base: base{pool: pool}}
}

const businessColumns = `id, name, created_at`

// Create inserts a business and fills in the generated id and timestamp.
func (r *BusinessRepository) Create(ctx context.Context, b *business.Business) error {
	const query = `
		INSERT INTO businesses (name, created_at)
		VALUES ($1, COALESCE($2, now()))
		RETURNING id, created_at`

	row := r.querier(ctx).QueryRow(ctx, query, b.Name, timeOrNil(b.CreatedAt))
	if err := row.Scan(&b.ID, &b.CreatedAt); err != nil {
		return fmt.Errorf("insert business: %w", err)
	}
	b.CreatedAt = b.CreatedAt.UTC()
	return nil
}

// GetByID reads one business, or business.ErrNotFound.
func (r *BusinessRepository) GetByID(ctx context.Context, id int64) (*business.Business, error) {
	query := `SELECT ` + businessColumns + ` FROM businesses WHERE id = $1`

	var found business.Business
	err := r.querier(ctx).QueryRow(ctx, query, id).Scan(&found.ID, &found.Name, &found.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, business.ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("select business: %w", err)
	}
	found.CreatedAt = found.CreatedAt.UTC()
	return &found, nil
}

// ExistsByID reports whether the business exists.
func (r *BusinessRepository) ExistsByID(ctx context.Context, id int64) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM businesses WHERE id = $1)`

	var exists bool
	if err := r.querier(ctx).QueryRow(ctx, query, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check business existence: %w", err)
	}
	return exists, nil
}

// List returns every business, alphabetically. The table is small by nature (one
// row per client company), so it is not paginated.
func (r *BusinessRepository) List(ctx context.Context) ([]*business.Business, error) {
	query := `SELECT ` + businessColumns + ` FROM businesses ORDER BY name ASC`

	rows, err := r.querier(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select businesses: %w", err)
	}
	defer rows.Close()

	businesses := make([]*business.Business, 0, 8)
	for rows.Next() {
		var found business.Business
		if err := rows.Scan(&found.ID, &found.Name, &found.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan business: %w", err)
		}
		found.CreatedAt = found.CreatedAt.UTC()
		businesses = append(businesses, &found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate businesses: %w", err)
	}
	return businesses, nil
}
