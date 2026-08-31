package pgxmock

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errClosed = errors.New("rows closed")

// An expectation reused via Times() must serve the full result set on every
// call, not just the first one.
func TestQueryRowsAreFreshOnEveryCall(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	eq := mock.ExpectQuery("SELECT").WillReturnRows(
		NewRows([]string{"id"}).AddRow(1).AddRow(2))
	eq.Times(3)

	for call := range 3 {
		rows, err := mock.Query(context.Background(), "SELECT id FROM t")
		assert.NoError(t, err, "call %d", call)

		var got []any
		for rows.Next() {
			var id any
			assert.NoError(t, rows.Scan(&id))
			got = append(got, id)
		}
		rows.Close()
		assert.NoError(t, rows.Err(), "call %d", call)
		assert.Equal(t, []any{1, 2}, got, "call %d must see every row", call)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Row-level and close errors must be reported on every call too, not consumed
// by the first one.
func TestQueryRowErrorsRepeatOnEveryCall(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	eq := mock.ExpectQuery("SELECT").WillReturnRows(
		NewRows([]string{"id"}).AddRow(1).CloseError(errClosed))
	eq.Times(2)

	for call := range 2 {
		rows, err := mock.Query(context.Background(), "SELECT id FROM t")
		assert.NoError(t, err, "call %d", call)
		for rows.Next() {
		}
		rows.Close()
		assert.ErrorIs(t, rows.Err(), errClosed, "call %d", call)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Multiple result sets are cloned independently as well.
func TestQueryMultipleRowSetsAreFresh(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	eq := mock.ExpectQuery("SELECT").WillReturnRows(
		NewRows([]string{"id"}).AddRow(1),
		NewRows([]string{"id"}).AddRow(2))
	eq.Times(2)

	for call := range 2 {
		rows, err := mock.Query(context.Background(), "SELECT id FROM t")
		assert.NoError(t, err, "call %d", call)
		assert.True(t, rows.Next(), "call %d must have a row", call)
		rows.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
