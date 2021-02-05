package main

import (
	"context"

	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Commit(context.Context) error
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
	Close(context.Context) error
}

func recordStats(db PgxIface, userID, productID int64) (err error) {
	tx, err := db.Begin(context.Background())
	if err != nil {
		return
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(context.Background())
		default:
			tx.Rollback(context.Background())
		}
	}()

	if _, err = tx.Exec(context.Background(), "UPDATE products SET views = views + 1"); err != nil {
		return
	}
	if _, err = tx.Exec(context.Background(), "INSERT INTO product_viewers (user_id, product_id) VALUES (?, ?)", userID, productID); err != nil {
		return
	}
	return
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgx.Connect(context.Background(), "root@/blog")
	if err != nil {
		panic(err)
	}
	defer db.Close(context.Background())

	if err = recordStats(db, 1 /*some user id*/, 5 /*some product id*/); err != nil {
		panic(err)
	}
}
