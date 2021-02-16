package main

import (
	"context"
	"fmt"
	"log"

	pgconn "github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

const orderPending = 0
const orderCancelled = 1

type User struct {
	ID       int     `sql:"u_id"`
	Username string  `sql:"username"`
	Balance  float64 `sql:"balance"`
}

type Order struct {
	ID          int     `sql:"o_id"`
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
SELECT %s, %s
FROM orders AS o
INNER JOIN users AS u ON o.buyer_id = u.id
WHERE o.id = ?
FOR UPDATE`,
		ColumnsAliased(order, "o"),
		ColumnsAliased(user, "u"))

	// fetch order to cancel
	rows, err := tx.Query(context.Background(), sql, id)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}

	defer rows.Close()
	// no rows, nothing to do
	if !rows.Next() {
		_ = tx.Rollback(context.Background())
		return
	}

	// read order
	err = ScanAliased(&order, rows, "o")
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}

	// ensure order status
	if order.Status != orderPending {
		_ = tx.Rollback(context.Background())
		return
	}

	// read user
	err = ScanAliased(&user, rows, "u")
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}
	rows.Close() // manually close before other prepared statements

	// refund order value
	sql = "UPDATE users SET balance = balance + ? WHERE id = ?"
	_, err = tx.Prepare(context.Background(), "balance_stmt", sql)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}

	_, err = tx.Exec(context.Background(), "balance_stmt", order.Value+order.ReservedFee, user.ID)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}

	// update order status
	order.Status = orderCancelled
	sql = "UPDATE orders SET status = ?, updated = NOW() WHERE id = ?"
	_, err = tx.Prepare(context.Background(), "order_stmt", sql)
	if err != nil {
		_ = tx.Rollback(context.Background())
		return
	}
	_, err = tx.Exec(context.Background(), "order_stmt", order.Status, order.ID)
	if err != nil {
		_ = tx.Rollback(context.Background())
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
