// Package business contains the business use cases and their ports. Businesses
// are mostly read-only for this platform: the dashboard needs them to populate the
// selector when creating a payment reference.
package business

import (
	"context"

	domain "paymentlinks/internal/domain/business"
)

// Repository is the persistence port for businesses. Implementations return
// domain.ErrNotFound when a row is missing.
type Repository interface {
	Create(ctx context.Context, b *domain.Business) error
	GetByID(ctx context.Context, id int64) (*domain.Business, error)
	List(ctx context.Context) ([]*domain.Business, error)
	// ExistsByID answers the only question the payment reference use cases ask
	// about a business, without paying for a full row read.
	ExistsByID(ctx context.Context, id int64) (bool, error)
}
