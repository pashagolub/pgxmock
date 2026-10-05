package pgxmock_test

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v6"
)

// userCopier is what importUsers needs from *pgx.Conn or *pgxpool.Pool.
type userCopier interface {
	CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, src pgx.CopyFromSource) (int64, error)
}

type user struct {
	name string
	age  int32
}

// importUsers is the code under test.
func importUsers(ctx context.Context, db userCopier, users []user) (int64, error) {
	return db.CopyFrom(ctx, pgx.Identifier{"users"}, []string{"name", "age"},
		pgx.CopyFromSlice(len(users), func(i int) ([]any, error) {
			return []any{users[i].name, users[i].age}, nil
		}))
}

func ExampleNewCopyRows() {
	mock, _ := pgxmock.NewConn()
	// the age is copied as an int32, compared as the same value the server would store
	mock.ExpectCopyFrom(pgx.Identifier{"users"}, []string{"name", "age"}).
		WithRows(pgxmock.NewCopyRows("name", "age").AddRow("alice", 30)).
		WillReturnResult(1)

	n, err := importUsers(context.Background(), mock, []user{{"alice", 30}})
	fmt.Println(n, err)
	// Output: 1 <nil>
}
