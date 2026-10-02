package pgxmock_test

import (
	"context"
	"fmt"

	"github.com/pashagolub/pgxmock/v5"
)

// pinger is what isAlive needs from *pgx.Conn or *pgxpool.Pool.
type pinger interface {
	Ping(ctx context.Context) error
}

// isAlive is the code under test.
func isAlive(ctx context.Context, db pinger) bool {
	return db.Ping(ctx) == nil
}

func ExampleErrorOnClosedConnOption() {
	mock, _ := pgxmock.NewConn(pgxmock.ErrorOnClosedConnOption())
	mock.ExpectClose()
	_ = mock.Close(context.Background())

	fmt.Println(isAlive(context.Background(), mock))
	// Output: false
}
