package main

import (
	"context"
	"fmt"
	"log"

	pgconn "github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

const ORDER_PENDING = 0
const ORDER_CANCELLED = 1

type User struct {
	Id       int     `sql:"id"`
	Username string  `sql:"username"`
	Balance  float64 `sql:"balance"`
}

type Order struct {
	Id          int     `sql:"id"`
	Value       float64 `sql:"value"`
	ReservedFee float64 `sql:"reserved_fee"`
	Status      int     `sql:"status"`
}

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
	Close(context.Context) error
}

func cancelOrder(id int, db PgxIface) (err error) {
	tx, err := db.Begin(context.Background())
	if err != nil {
		return
	}

	var order Order
	var user User
	sql := fmt.Sprintf(`
SELECT o.*, u.*
FROM orders AS o
INNER JOIN users AS u ON o.buyer_id = u.id
WHERE o.id = $1
FOR UPDATE`)

	// fetch order to cancel
	rows, err := tx.Query(context.Background(), sql, id)
	if err != nil {
		tx.Rollback(context.Background())
		return
	}

	defer rows.Close()
	// no rows, nothing to do
	if !rows.Next() {
		tx.Rollback(context.Background())
		return
	}

	// read order
	// TODO:
	// err = sqlstruct.ScanAliased(&order, rows, "o")
	if err != nil {
		tx.Rollback(context.Background())
		return
	}

	// ensure order status
	if order.Status != ORDER_PENDING {
		tx.Rollback(context.Background())
		return
	}

	// read user
	// TODO:
	// err = sqlstruct.ScanAliased(&user, rows, "u")
	if err != nil {
		tx.Rollback(context.Background())
		return
	}
	rows.Close() // manually close before other prepared statements

	// refund order value
	sql = "UPDATE users SET balance = balance + ? WHERE id = ?"
	_, err = tx.Prepare(context.Background(), "balance_stmt", sql)
	if err != nil {
		tx.Rollback(context.Background())
		return
	}

	_, err = tx.Exec(context.Background(), "balance_stmt", order.Value+order.ReservedFee, user.Id)
	if err != nil {
		tx.Rollback(context.Background())
		return
	}

	// update order status
	order.Status = ORDER_CANCELLED
	sql = "UPDATE orders SET status = ?, updated = NOW() WHERE id = ?"
	_, err = tx.Prepare(context.Background(), sql, "order_stmt")
	if err != nil {
		tx.Rollback(context.Background())
		return
	}
	_, err = tx.Exec(context.Background(), "order_stmt", order.Status, order.Id)
	if err != nil {
		tx.Rollback(context.Background())
		return
	}
	return tx.Commit(context.Background())
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgx.Connect(context.Background(), "postgres://postgres@localhost/orders")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(context.Background())
	err = cancelOrder(1, db)
	if err != nil {
		log.Fatal(err)
	}
}
