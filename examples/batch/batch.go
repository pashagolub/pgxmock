package main

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Close()
}

func selectBatch(db PgxIface) (err error) {
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

	batch := &pgx.Batch{}
	batch.Queue("insert into ledger(description, amount) values($1, $2)", "q1", 1)
	batch.Queue("insert into ledger(description, amount) values($1, $2)", "q2", 2)
	batch.Queue("insert into ledger(description, amount) values($1, $2)", "q3", 3)
	batch.Queue("select id, description, amount from ledger order by id")
	batch.Queue("select id, description, amount from ledger order by id")
	batch.Queue("select * from ledger where false")
	batch.Queue("select sum(amount) from ledger")

	if br := tx.SendBatch(context.Background(), batch); br == nil {
		return errors.New("SendBatch returns a NIL object")
	}

	// TODO : call BatchResults.Exec method
	// _, err = br.Exec()
	return
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgxpool.New(context.Background(), "postgres://rolname@hostname/dbname")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err = selectBatch(db); err != nil {
		panic(err)
	}
}
