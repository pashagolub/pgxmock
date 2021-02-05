package pgxmock

import (
	"context"

	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

type statement struct {
	conn  *pgxmock
	ex    *ExpectedPrepare
	query string
}

func (stmt *statement) Close() error {
	stmt.ex.wasClosed = true
	return stmt.ex.closeErr
}

func (stmt *statement) NumInput() int {
	return -1
}

// Deprecated: Drivers should implement ExecerContext instead.
func (stmt *statement) Exec(args []interface{}) (pgconn.CommandTag, error) {
	return stmt.conn.Exec(context.Background(), stmt.query, convertValueToNamedValue(args))
}

// Deprecated: Drivers should implement StmtQueryContext instead (or additionally).
func (stmt *statement) Query(args []interface{}) (pgx.Rows, error) {
	return stmt.conn.Query(context.Background(), stmt.query, convertValueToNamedValue(args))
}

func convertValueToNamedValue(args []interface{}) []interface{} {
	return nil
	// namedArgs := make([]driver.NamedValue, len(args))
	// for i, v := range args {
	// 	namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	// }
	// return namedArgs
}
