package pgxmock_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
)

// userInserter is what createUser needs from *pgx.Conn or *pgxpool.Pool.
type userInserter interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

var errUserExists = errors.New("user already exists")

// createUser is the code under test.
func createUser(ctx context.Context, db userInserter, email string) error {
	_, err := db.Exec(ctx, "INSERT INTO users(email) VALUES ($1)", email)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errUserExists
	}
	return err
}

func ExampleNewPgError() {
	mock, _ := pgxmock.NewConn()
	mock.ExpectExec("INSERT INTO users").
		WithArgs("a@b.c").
		WillReturnError(pgxmock.NewPgError("23505", "duplicate key value violates unique constraint"))

	err := createUser(context.Background(), mock, "a@b.c")
	fmt.Println(err)
	// Output: user already exists
}
