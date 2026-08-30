package pgxmock

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

type optUser struct {
	ID   int
	Name string
}

// pgx consumes QueryExecMode, QueryResultFormats, QueryResultFormatsByOID and
// QueryRewriter values that precede the real arguments, so an expectation must
// describe only the query parameters.
func TestQueryConsumesLeadingOptions(t *testing.T) {
	for name, args := range map[string][]any{
		"exec mode":                {pgx.QueryExecModeSimpleProtocol, 7},
		"result formats":           {pgx.QueryResultFormats{1}, 7},
		"result formats by OID":    {pgx.QueryResultFormatsByOID{25: 1}, 7},
		"exec mode then formats":   {pgx.QueryExecModeExec, pgx.QueryResultFormats{1}, 7},
		"no options":               {7},
		"formats then exec mode":   {pgx.QueryResultFormats{1}, pgx.QueryExecModeExec, 7},
		"every option in sequence": {pgx.QueryResultFormats{1}, pgx.QueryResultFormatsByOID{25: 1}, pgx.QueryExecModeExec, 7},
	} {
		t.Run(name, func(t *testing.T) {
			mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
			assert.NoError(t, err)
			mock.ExpectQuery("SELECT name FROM users WHERE id=$1").
				WithArgs(7).
				WillReturnRows(NewRows([]string{"name"}).AddRow("bob"))

			rows, err := mock.Query(context.Background(),
				"SELECT name FROM users WHERE id=$1", args...)
			assert.NoError(t, err)
			rows.Close()
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Exec accepts a smaller option set than Query: pgx does not strip result
// formats there, so neither may the mock.
func TestExecConsumesOnlyItsOwnOptions(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)
	mock.ExpectExec("DELETE FROM users WHERE id=$1").
		WithArgs(7).
		WillReturnResult(NewResult("DELETE", 1))

	_, err = mock.Exec(context.Background(), "DELETE FROM users WHERE id=$1",
		pgx.QueryExecModeSimpleProtocol, 7)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	mock2, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)
	mock2.ExpectExec("DELETE FROM users WHERE id=$1").
		WithArgs(7).
		WillReturnResult(NewResult("DELETE", 1))

	// pgx would pass QueryResultFormats to Exec as a real parameter
	_, err = mock2.Exec(context.Background(), "DELETE FROM users WHERE id=$1",
		pgx.QueryResultFormats{1}, 7)
	assert.ErrorContains(t, err, "expected 1, but got 2 arguments")
}

// A rewriter combined with an exec mode used to be missed entirely, because
// the rewriter was only looked for when it was the single argument.
func TestQueryRewriterAfterExecMode(t *testing.T) {
	type queryCall struct {
		sql  string
		args []any
	}
	const namedSQL = "SELECT name FROM users WHERE id=@id"
	const structSQL = "SELECT name FROM users WHERE id=@ID"

	for name, call := range map[string]queryCall{
		"named args alone": {namedSQL, []any{pgx.NamedArgs{"id": 7}}},
		"exec mode then named args": {namedSQL,
			[]any{pgx.QueryExecModeExec, pgx.NamedArgs{"id": 7}}},
		"formats then named args": {namedSQL,
			[]any{pgx.QueryResultFormats{1}, pgx.NamedArgs{"id": 7}}},
		"strict named args": {namedSQL,
			[]any{pgx.QueryExecModeExec, pgx.StrictNamedArgs{"id": 7}}},
		"struct args": {structSQL, []any{pgx.StructArgs(optUser{ID: 7})}},
		"exec mode then struct args": {structSQL,
			[]any{pgx.QueryExecModeExec, pgx.StructArgs(optUser{ID: 7})}},
	} {
		t.Run(name, func(t *testing.T) {
			mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
			assert.NoError(t, err)
			mock.ExpectQuery("SELECT").
				WithArgs(7).
				WillReturnRows(NewRows([]string{"name"}).AddRow("bob"))

			rows, err := mock.Query(context.Background(), call.sql, call.args...)
			assert.NoError(t, err)
			rows.Close()
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// The expectation itself may be written with a rewriter, alongside options.
func TestExpectedArgsAreRewrittenToo(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectQuery("SELECT").
		WithArgs(pgx.QueryExecModeExec, pgx.NamedArgs{"id": 7}).
		WillReturnRows(NewRows([]string{"name"}).AddRow("bob"))

	rows, err := mock.Query(context.Background(),
		"SELECT name FROM users WHERE id=@id", pgx.NamedArgs{"id": 7})
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// WithRewrittenSQL still sees the SQL the rewriter produced.
func TestRewrittenSQLWithLeadingExecMode(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectQuery("SELECT").
		WithArgs(7).
		WithRewrittenSQL(`SELECT name FROM users WHERE id=\$1`).
		WillReturnRows(NewRows([]string{"name"}).AddRow("bob"))

	rows, err := mock.Query(context.Background(),
		"SELECT name FROM users WHERE id=@id",
		pgx.QueryExecModeExec, pgx.NamedArgs{"id": 7})
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A batch queued query accepts a rewriter, but not an exec mode: pgx passes
// anything else straight through as a parameter.
func TestBatchConsumesRewriterOnly(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	eb := mock.ExpectBatch()
	eb.ExpectQuery("SELECT").WithArgs(7).
		WillReturnRows(NewRows([]string{"name"}).AddRow("bob"))

	batch := &pgx.Batch{}
	batch.Queue("SELECT name FROM users WHERE id=@id", pgx.NamedArgs{"id": 7})

	br := mock.SendBatch(context.Background(), batch)
	rows, err := br.Query()
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, br.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}
