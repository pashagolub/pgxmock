package pgxmock

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

var copyTable = pgx.Identifier{"users"}
var copyColumns = []string{"name", "age"}

func TestCopyFromMatchingRows(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", 30}, []any{"bob", 40}).
		WillReturnResult(2)

	n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}, {"bob", 40}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCopyFromWrongValue(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", 30}).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 31}}))
	assert.ErrorContains(t, err, "value 1 of row 0")
	assert.ErrorContains(t, err, "30")
	assert.ErrorContains(t, err, "31")
}

func TestCopyFromWrongRowCount(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", 30}, []any{"bob", 40}).
		WillReturnResult(2)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "expected 2 row(s) to be copied, but got 1")
}

func TestCopyFromWrongValueCount(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", 30}).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice"}}))
	assert.ErrorContains(t, err, "row 0 expected 2 value(s), but got 1")
}

// Argument matchers stand in for values a test cannot predict.
func TestCopyFromWithArgumentMatcher(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, []string{"name", "created"}).
		WithRows([]any{"alice", AnyArg()}).
		WillReturnResult(1)

	n, err := mock.CopyFrom(context.Background(), copyTable, []string{"name", "created"},
		pgx.CopyFromRows([][]any{{"alice", time.Now()}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCopyFromMatcherMismatch(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", neverMatches{}}).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "could not match value 1 of row 0")
}

type neverMatches struct{}

func (neverMatches) Match(any) bool { return false }

// Without WithRows the copied data is still not inspected, as before.
func TestCopyFromWithoutRowExpectation(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).WillReturnResult(2)

	n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"anything", 1}, {"at", 2}, {"all", 3}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// An empty copy can be asserted too, distinctly from not asserting at all.
func TestCopyFromExpectingNoRows(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows().
		WillReturnResult(0)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "expected 0 row(s) to be copied, but got 1")
}

func TestCopyFromStringIncludesRows(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)
	e := mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows([]any{"alice", 30})

	assert.Contains(t, e.String(), "matches 1 row(s)")
	assert.Contains(t, e.String(), "alice")
}
