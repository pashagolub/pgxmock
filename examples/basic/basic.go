package main

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Close()
}

func recordStats(db PgxIface, userID, productID int) (err error) {
	tx, err := db.Begin(context.Background())
	if err != nil {
		return
	}
	defer func() {
		switch err {
		case nil:
			err = tx.Commit(context.Background())
		default:
			_ = tx.Rollback(context.Background())
		}
	}()
	sql := "UPDATE products SET views = views + 1"
	if _, err = tx.Exec(context.Background(), sql); err != nil {
		return
	}
	sql = "INSERT INTO product_viewers (user_id, product_id) VALUES ($1, $2)"
	if _, err = tx.Exec(context.Background(), sql, userID, productID); err != nil {
		return
	}
	return
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgxpool.New(context.Background(), "postgres://rolname@hostname/dbname")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err = recordStats(db, 1 /*some user id*/, 5 /*some product id*/); err != nil {
		panic(err)
	}
}
