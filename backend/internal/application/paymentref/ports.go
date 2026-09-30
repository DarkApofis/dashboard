// Package paymentref contains the payment reference use cases and the ports they
// depend on.
//
// The ports are declared here, in the layer that consumes them, not in the layer
// that implements them: use cases receive interfaces, so they can be driven by
// in-memory fakes in tests and by PostgreSQL in production without changing a
// line. The domain package is imported as `domain` to keep both layers readable
// under the same name.
package paymentref

import (
	"context"
	"time"

	domain "paymentlinks/internal/domain/paymentref"
)

// Repository is the persistence port for payment references.
//
// Implementations must translate driver-level failures into domain errors:
// domain.ErrNotFound when a row is missing, domain.ErrDuplicateReference on a
// unique violation of external_reference, domain.ErrConcurrentUpdate when a
// guarded update matches no row.
type Repository interface {
	// Create persists a new reference and assigns its ID.
	Create(ctx context.Context, ref *domain.PaymentReference) error

	// GetByID reads a reference, or domain.ErrNotFound.
	GetByID(ctx context.Context, id int64) (*domain.PaymentReference, error)

	// GetByExternalReference reads a reference by the identifier the financial
	// provider uses, or domain.ErrNotFound.
	GetByExternalReference(ctx context.Context, externalReference string) (*domain.PaymentReference, error)

	// LockByID reads a reference FOR UPDATE. It must be called inside a
	// transaction (see TxManager), otherwise the lock is released immediately.
	LockByID(ctx context.Context, id int64) (*domain.PaymentReference, error)

	// LockByExternalReference is LockByID keyed by external reference; it is what
	// serialises concurrent webhook deliveries for the same reference.
	LockByExternalReference(ctx context.Context, externalReference string) (*domain.PaymentReference, error)

	// List returns a filtered page. now is passed explicitly so that the status
	// filter can be applied to the effective status (a pending reference past its
	// expiration date counts as expired) without the repository owning a clock.
	List(ctx context.Context, filter Filter, now time.Time) (Page, error)

	// UpdateWithStatusGuard persists the mutable state of a reference only if its
	// stored status still equals expected, which gives optimistic concurrency
	// without a version column. It returns domain.ErrConcurrentUpdate otherwise.
	UpdateWithStatusGuard(ctx context.Context, ref *domain.PaymentReference, expected domain.Status) error

	// ExpireDue materialises expirations in bounded batches and returns how many
	// rows it moved. It is idempotent: running it twice changes nothing.
	ExpireDue(ctx context.Context, now time.Time, limit int) (int64, error)
}

// BusinessFinder is the slice of the business repository that this package needs:
// creating a reference only has to know whether the business exists. Declaring the
// narrow interface here, instead of importing the full business repository port,
// keeps the two use case packages independent of each other.
type BusinessFinder interface {
	ExistsByID(ctx context.Context, id int64) (bool, error)
}

// WebhookEventRepository is the persistence port for provider notifications. The
// unique constraint on event_id is the idempotency mechanism, so Insert returning
// domain.ErrDuplicateEvent is the signal that the event was already handled.
type WebhookEventRepository interface {
	// Insert records a delivery, or returns domain.ErrDuplicateEvent if that
	// event_id is already recorded. Implementations must leave the surrounding
	// transaction usable in that case, so the caller can read the original outcome.
	Insert(ctx context.Context, event *domain.WebhookEvent) error
	// Finalize stamps the outcome and the resolved reference on an event already
	// inserted in this transaction.
	Finalize(ctx context.Context, event *domain.WebhookEvent) error
	GetByEventID(ctx context.Context, eventID string) (*domain.WebhookEvent, error)
}

// TxManager runs fn inside a single database transaction. The transaction travels
// in the context, so use cases compose repository calls atomically without ever
// importing a database driver. Nested calls join the outer transaction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Clock is the time port. Injecting it keeps expiration logic testable.
type Clock interface {
	Now() time.Time
}
