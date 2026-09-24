package main

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

type PgxIface interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

type user struct {
	Name string
	Age  int
}

// importUsers bulk loads users with the COPY protocol.
func importUsers(ctx context.Context, db PgxIface, users []user) error {
	n, err := db.CopyFrom(ctx, pgx.Identifier{"users"}, []string{"name", "age"},
		pgx.CopyFromSlice(len(users), func(i int) ([]any, error) {
			return []any{users[i].Name, users[i].Age}, nil
		}))
	if err != nil {
		return err
	}
	if n != int64(len(users)) {
		return fmt.Errorf("copied %d users out of %d", n, len(users))
	}
	return nil
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgxpool.New(context.Background(), "postgres://rolname@hostname/dbname")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := importUsers(context.Background(), db, []user{{"alice", 30}, {"bob", 40}}); err != nil {
		panic(err)
	}
}
