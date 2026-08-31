package pgxmock

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// Every database operation must report pgconn.ErrConnClosed once the mocked
// connection has been closed.
func TestClosedConnRejectsOperations(t *testing.T) {
	for name, call := range map[string]func(m PgxConnIface) error{
		"Query": func(m PgxConnIface) error {
			rows, err := m.Query(context.Background(), "SELECT 1")
			assert.NotNil(t, rows)
			rows.Close()
			return err
		},
		"QueryRow": func(m PgxConnIface) error {
			return m.QueryRow(context.Background(), "SELECT 1").Scan(new(int))
		},
		"Exec": func(m PgxConnIface) error {
			_, err := m.Exec(context.Background(), "DELETE FROM t")
			return err
		},
		"Begin": func(m PgxConnIface) error {
			_, err := m.Begin(context.Background())
			return err
		},
		"BeginTx": func(m PgxConnIface) error {
			_, err := m.BeginTx(context.Background(), pgx.TxOptions{})
			return err
		},
		"Commit":   func(m PgxConnIface) error { return m.Commit(context.Background()) },
		"Rollback": func(m PgxConnIface) error { return m.Rollback(context.Background()) },
		"Ping":     func(m PgxConnIface) error { return m.Ping(context.Background()) },
		"Prepare": func(m PgxConnIface) error {
			_, err := m.Prepare(context.Background(), "s", "SELECT 1")
			return err
		},
		"Deallocate":    func(m PgxConnIface) error { return m.Deallocate(context.Background(), "s") },
		"DeallocateAll": func(m PgxConnIface) error { return m.DeallocateAll(context.Background()) },
		"CopyFrom": func(m PgxConnIface) error {
			_, err := m.CopyFrom(context.Background(), pgx.Identifier{"t"}, []string{"a"},
				pgx.CopyFromRows([][]any{{1}}))
			return err
		},
		"SendBatch": func(m PgxConnIface) error {
			batch := &pgx.Batch{}
			batch.Queue("SELECT 1")
			return m.SendBatch(context.Background(), batch).Close()
		},
	} {
		t.Run(name, func(t *testing.T) {
			mock, err := NewConn(
				QueryMatcherOption(QueryMatcherAny),
				ErrorOnClosedConnOption())
			assert.NoError(t, err)
			mock.ExpectClose()
			assert.NoError(t, mock.Close(context.Background()))

			assert.ErrorIs(t, call(mock), pgconn.ErrConnClosed)
		})
	}
}

// Without the option the previous, permissive behaviour is preserved.
func TestClosedConnIsPermissiveByDefault(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectClose()
	mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}).AddRow(1))

	assert.NoError(t, mock.Close(context.Background()))

	rows, err := mock.Query(context.Background(), "SELECT 1")
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A closed pool behaves the same way.
func TestClosedPoolRejectsOperations(t *testing.T) {
	mock, err := NewPool(
		QueryMatcherOption(QueryMatcherAny),
		ErrorOnClosedConnOption())
	assert.NoError(t, err)
	mock.ExpectClose()
	mock.Close()

	_, err = mock.Exec(context.Background(), "DELETE FROM t")
	assert.ErrorIs(t, err, pgconn.ErrConnClosed)
}

// Closing is idempotent and operations stay rejected.
func TestClosedConnStaysClosed(t *testing.T) {
	mock, err := NewConn(
		QueryMatcherOption(QueryMatcherAny),
		ErrorOnClosedConnOption())
	assert.NoError(t, err)
	ec := mock.ExpectClose()
	ec.Times(2)

	assert.NoError(t, mock.Close(context.Background()))
	assert.NoError(t, mock.Close(context.Background()))
	assert.ErrorIs(t, mock.Ping(context.Background()), pgconn.ErrConnClosed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A connection view taken from a pool observes the pool being closed.
func TestAsConnObservesPoolClose(t *testing.T) {
	pool, err := NewPool(
		QueryMatcherOption(QueryMatcherAny),
		ErrorOnClosedConnOption())
	assert.NoError(t, err)

	conn := pool.AsConn()
	pool.ExpectClose()
	pool.Close()

	assert.ErrorIs(t, conn.Ping(context.Background()), pgconn.ErrConnClosed)
}
