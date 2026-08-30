package pgxmock

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

// Rows explicitly attached to an expectation are still returned alongside the
// error, so tests that arrange both keep working.
func TestQueryReturnsRowsAndErrorTogether(t *testing.T) {
	errBoom := errors.New("boom")
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	eq := mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}).AddRow(1))
	eq.WillReturnError(errBoom)

	rows, err := mock.Query(context.Background(), "SELECT id FROM t")
	assert.ErrorIs(t, err, errBoom)
	assert.NotNil(t, rows)
	assert.True(t, rows.Next(), "the arranged rows must still be readable")
	rows.Close()
}
