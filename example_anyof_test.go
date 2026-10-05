package pgxmock_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v6"
)

// orderUpdater is what setOrderPaid needs from *pgx.Conn or *pgxpool.Pool.
type orderUpdater interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// setOrderPaid is the code under test.
func setOrderPaid(ctx context.Context, db orderUpdater, id int, paid bool) error {
	status := "pending"
	if paid {
		status = "paid"
	}
	_, err := db.Exec(ctx, "UPDATE orders SET status = $1, updated = $2 WHERE id = $3", status, time.Now(), id)
	return err
}

func ExampleAnyOf() {
	mock, _ := pgxmock.NewConn()
	// the status depends on the input and the timestamp on the clock
	mock.ExpectExec("UPDATE orders").
		WithArgs(pgxmock.AnyOf("pending", "paid"), pgxmock.OfType[time.Time](), 42).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := setOrderPaid(context.Background(), mock, 42, true)
	fmt.Println(err)
	// Output: <nil>
}
