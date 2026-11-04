package pgxmock_test

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
)

// batchSender is what addUser needs from *pgx.Conn or *pgxpool.Pool.
type batchSender interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// addUser is the code under test. It starts the transaction in the first query of the batch.
func addUser(ctx context.Context, db batchSender, name string) error {
	b := &pgx.Batch{}
	b.BeginTx(pgx.TxOptions{})
	b.Queue("INSERT INTO users (name) VALUES ($1)", name)
	br := db.SendBatch(ctx, b)
	if err := br.Close(); err != nil {
		_ = br.Tx().Rollback(ctx)
		return err
	}
	return br.Tx().Commit(ctx)
}

func ExampleExpectedBatch_beginTx() {
	mock, _ := pgxmock.NewConn()
	// the begin query comes first, and the mock itself is the transaction BatchResults.Tx returns
	eb := mock.ExpectBatch()
	eb.ExpectExec("begin").WillReturnResult(pgxmock.NewResult("BEGIN", 0))
	eb.ExpectExec("INSERT INTO users").WithArgs("alice").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	fmt.Println(addUser(context.Background(), mock, "alice"))
	// Output: <nil>
}
