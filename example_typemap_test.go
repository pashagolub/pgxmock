package pgxmock_test

import (
	"context"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v6"
)

// orderReader is what orderStatus needs from *pgx.Conn or *pgxpool.Pool.
type orderReader interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// orderStatus is the code under test.
func orderStatus(ctx context.Context, db orderReader, id int) (string, error) {
	var status string
	err := db.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", id).Scan(&status)
	return status, err
}

func ExamplePgxConnIface_typeMap() {
	const statusOID = 100000
	mock, _ := pgxmock.NewConn()
	// register the custom type the way it is done on a *pgx.Conn
	mock.TypeMap().RegisterType(&pgtype.Type{Name: "status", OID: statusOID, Codec: &pgtype.EnumCodec{}})

	col := mock.NewColumn("status")
	col.DataTypeOID = statusOID
	mock.ExpectQuery("SELECT status").
		WithArgs(7).
		WillReturnRows(mock.NewRowsWithColumnDefinition(*col).AddRow("active"))

	status, err := orderStatus(context.Background(), mock, 7)
	fmt.Println(status, err)
	// Output: active <nil>
}
