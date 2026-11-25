package port

import "context"

// TxManager runs a function atomically. Repositories called with the ctx
// passed to fn take part in the transaction; if fn returns an error the
// transaction is rolled back and that error is returned.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
