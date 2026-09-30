package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	apppaymentref "paymentlinks/internal/application/paymentref"
)

// txKey carries the ambient transaction in the context. It is unexported so no
// other package can put or read a pgx.Tx there: the only way to open one is
// TxManager.WithTx, and the only consumers are the repositories in this package.
type txKey struct{}

// Querier is the subset of pgx shared by *pgxpool.Pool and pgx.Tx. Repositories
// take it from the context so the very same method works inside and outside a
// transaction without any branching in the use cases.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager implements the application TxManager port.
type TxManager struct {
	pool *pgxpool.Pool
}

// Compile-time check that the port is satisfied.
var _ apppaymentref.TxManager = (*TxManager)(nil)

// NewTxManager builds a TxManager over the given pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx runs fn inside a transaction, committing when it returns nil and rolling
// back on any error or panic. If the context already carries a transaction, fn
// joins it instead of opening a nested one, so use cases can be composed freely.
func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recovered := recover(); recovered != nil {
			// Roll back on a detached context: the incoming one may already be
			// cancelled, and a leaked transaction is worse than a slow one.
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(recovered)
		}
	}()

	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(context.WithoutCancel(ctx)); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// base is embedded by every repository and resolves which Querier to use.
type base struct {
	pool *pgxpool.Pool
}

func (b base) querier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return b.pool
}
