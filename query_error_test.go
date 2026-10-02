package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryErrorIsReportedByRows(t *testing.T) {
	mock, _ := NewConn()
	mock.ExpectQuery("SELECT").
		WillReturnRows(NewRows([]string{"id"}).AddRow(1)).
		WillDelayFor(time.Second)

	timeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	rows, _ := mock.Query(timeout, "SELECT")
	_, err := pgx.CollectRows(rows, pgx.RowTo[int])
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// pgx.Conn.Query always returns a usable pgx.Rows, even when it also returns
// an error, so that the idiomatic `rows, err := Query(); defer rows.Close()`
// does not panic. The mock must behave the same way.
func TestQueryReturnsUsableRowsOnError(t *testing.T) {
	errBoom := errors.New("boom")

	for name, arrange := range map[string]func(m PgxConnIface){
		"expectation returns an error": func(m PgxConnIface) {
			m.ExpectQuery("SELECT").WillReturnError(errBoom)
		},
		"no matching expectation": func(_ PgxConnIface) {},
	} {
		t.Run(name, func(t *testing.T) {
			mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
			assert.NoError(t, err)
			arrange(mock)

			rows, err := mock.Query(context.Background(), "SELECT id FROM t")
			assert.Error(t, err)
			assert.NotNil(t, rows, "Query must never return a nil pgx.Rows")

			// None of these may panic.
			assert.False(t, rows.Next())
			assert.Error(t, rows.Err())
			assert.Error(t, rows.Scan(new(int)))
			assert.Nil(t, rows.FieldDescriptions())
			assert.Nil(t, rows.RawValues())
			_, valErr := rows.Values()
			assert.Error(t, valErr)
			rows.Close()
		})
	}
}

func TestBatchQueryErrorReturnsRows(t *testing.T) {
	mock, _ := NewConn()
	mock.ExpectBatch()

	br := mock.SendBatch(ctx, &pgx.Batch{})
	rows, err := br.Query()
	assert.ErrorContains(t, err, "no more queries in batch")
	require.NotNil(t, rows, "pgx never returns nil rows")
	rows.Close()
}

// As in pgx, rows returned with an error yield nothing and report the error.
func TestQueryReturnsRowsAndErrorTogether(t *testing.T) {
	errBoom := errors.New("boom")
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	eq := mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}).AddRow(1))
	eq.WillReturnError(errBoom)

	rows, err := mock.Query(context.Background(), "SELECT id FROM t")
	assert.ErrorIs(t, err, errBoom)
	assert.NotNil(t, rows)
	assert.False(t, rows.Next())
	assert.ErrorIs(t, rows.Err(), errBoom)
	rows.Close()
}

func TestClosingFailedQueryRowsSatisfiesRowsWillBeClosed(t *testing.T) {
	errQuery := errors.New("query")
	mock, _ := NewConn()
	mock.ExpectQuery("SELECT").WillReturnError(errQuery).RowsWillBeClosed()

	rows, err := mock.Query(ctx, "SELECT")
	assert.ErrorIs(t, err, errQuery)
	rows.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFailedQueryRowClosesRows(t *testing.T) {
	mock, _ := NewConn()
	mock.ExpectQuery("SELECT").WillReturnError(errors.New("query")).RowsWillBeClosed()

	var id int
	_ = mock.QueryRow(ctx, "SELECT").Scan(&id)
	assert.NoError(t, mock.ExpectationsWereMet())
}
