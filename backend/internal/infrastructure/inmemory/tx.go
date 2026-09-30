package inmemory

import (
	"context"

	apppaymentref "paymentlinks/internal/application/paymentref"
)

// TxManager satisfies the transaction port by simply running the function.
//
// There is nothing to commit or roll back in memory, so a test using it verifies
// what the use case decides, not that the decision was atomic. Rollback and the
// SELECT ... FOR UPDATE race are properties of the PostgreSQL adapter and belong in
// an integration test against a real database.
type TxManager struct{}

var _ apppaymentref.TxManager = TxManager{}

// NewTxManager builds the no-op transaction manager.
func NewTxManager() TxManager { return TxManager{} }

// WithTx runs fn.
func (TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
