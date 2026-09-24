package main

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

type PgxIface interface {
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
}

// transfer moves amount from one account to another in a single round trip
// and returns the new balance of the source account.
func transfer(ctx context.Context, db PgxIface, from, to, amount int) (balance int, err error) {
	b := &pgx.Batch{}
	b.Queue("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, from).
		Exec(func(ct pgconn.CommandTag) error {
			if ct.RowsAffected() != 1 {
				return fmt.Errorf("account %d not found", from)
			}
			return nil
		})
	b.Queue("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, to)
	b.Queue("SELECT balance FROM accounts WHERE id = $1", from).
		QueryRow(func(row pgx.Row) error {
			return row.Scan(&balance)
		})
	err = db.SendBatch(ctx, b).Close()
	return
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgxpool.New(context.Background(), "postgres://rolname@hostname/dbname")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	balance, err := transfer(context.Background(), db, 1, 2, 100)
	if err != nil {
		panic(err)
	}
	fmt.Println("balance left:", balance)
}
