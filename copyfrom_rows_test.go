package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

var copyTable = pgx.Identifier{"users"}
var copyColumns = []string{"name", "age"}

var errCopyRejected = errors.New("copy rejected")

func TestCopyFromMatchingRows(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).
			AddRow("alice", 30).
			AddRow("bob", 40)).
		WillReturnResult(2)

	n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}, {"bob", 40}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Values are compared as a server would compare them, by what they encode to,
// so the Go type a CopyFromSource happens to yield does not matter.
func TestCopyFromComparesByEncodedValue(t *testing.T) {
	instant := time.Now()

	for _, tc := range []struct {
		name             string
		expected, copied any
	}{
		{"int against int32", 30, int32(30)},
		{"int against int64", 30, int64(30)},
		{"int32 against int", int32(30), 30},
		{"string against bytes", "alice", []byte("alice")},
		{"time without its monotonic reading", instant, time.Unix(0, instant.UnixNano())},
		{"time in another location", instant, instant.UTC()},
		{"nil against nil", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := NewConn()
			assert.NoError(t, err)

			mock.ExpectCopyFrom(copyTable, []string{"value"}).
				WithRows(NewCopyRows("value").AddRow(tc.expected)).
				WillReturnResult(1)

			_, err = mock.CopyFrom(context.Background(), copyTable, []string{"value"},
				pgx.CopyFromRows([][]any{{tc.copied}}))
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCopyFromWrongValue(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("alice", 30)).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 31}}))
	assert.ErrorContains(t, err, "value 1 of row 0")
	assert.ErrorContains(t, err, "30")
	assert.ErrorContains(t, err, "31")
}

// A test that only inspects the copied count must not pass on wrong data.
func TestCopyFromMismatchFailsExpectationsWereMet(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("alice", 30)).
		WillReturnResult(1)

	_, _ = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"mallory", 30}}))
	assert.ErrorContains(t, mock.ExpectationsWereMet(), "value 0 of row 0")
}

// A copy that fails reports no rows, the way pgx reports the RowsAffected of a
// command tag it never received.
func TestCopyFromReportsNoRowsOnFailure(t *testing.T) {
	t.Run("row mismatch", func(t *testing.T) {
		mock, err := NewConn()
		assert.NoError(t, err)

		mock.ExpectCopyFrom(copyTable, copyColumns).
			WithRows(NewCopyRows(copyColumns...).AddRow("alice", 30)).
			WillReturnResult(99)

		n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
			pgx.CopyFromRows([][]any{{"mallory", 30}}))
		assert.Error(t, err)
		assert.EqualValues(t, 0, n)
	})

	t.Run("rejected by the server", func(t *testing.T) {
		mock, err := NewConn()
		assert.NoError(t, err)

		mock.ExpectCopyFrom(copyTable, copyColumns).
			WillReturnResult(99).
			WillReturnError(errCopyRejected)

		n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
			pgx.CopyFromRows([][]any{{"alice", 30}}))
		assert.ErrorIs(t, err, errCopyRejected)
		assert.EqualValues(t, 0, n)
	})

	t.Run("source fails", func(t *testing.T) {
		mock, err := NewConn()
		assert.NoError(t, err)

		mock.ExpectCopyFrom(copyTable, copyColumns).WillReturnResult(99)

		n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
			&failingCopySource{err: errCopyRejected})
		assert.ErrorIs(t, err, errCopyRejected)
		assert.EqualValues(t, 0, n)
	})
}

// failingCopySource yields one row and then reports an error, the way a source
// reading from somewhere that broke mid-copy does.
type failingCopySource struct {
	err  error
	read bool
}

func (s *failingCopySource) Next() bool { return !s.read }
func (s *failingCopySource) Err() error { return s.err }
func (s *failingCopySource) Values() ([]any, error) {
	s.read = true
	return []any{"alice", 30}, nil
}

func TestCopyFromWrongRowCount(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).
			AddRow("alice", 30).
			AddRow("bob", 40)).
		WillReturnResult(2)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "expected 2 row(s) to be copied, but got 1")
}

func TestCopyFromWrongValueCount(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("alice", 30)).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice"}}))
	assert.ErrorContains(t, err, "row 0 expected 2 value(s), but got 1")
}

// A row of the wrong width is a mistake in the test, reported where it is made.
func TestCopyRowsAddRowPanicsOnWrongWidth(t *testing.T) {
	assert.Panics(t, func() {
		NewCopyRows(copyColumns...).AddRow("alice")
	})
}

// So are rows assembled over columns the expectation does not have.
func TestCopyFromWithRowsPanicsOnWrongColumns(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	assert.PanicsWithValue(t,
		"CopyFrom: expected rows have columns [age name], but the expected columns are [name age]",
		func() {
			mock.ExpectCopyFrom(copyTable, copyColumns).
				WithRows(NewCopyRows("age", "name").AddRow(30, "alice"))
		})
}

// Argument matchers stand in for values a test cannot predict.
func TestCopyFromWithArgumentMatcher(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	columns := []string{"name", "created"}
	mock.ExpectCopyFrom(copyTable, columns).
		WithRows(NewCopyRows(columns...).AddRow("alice", AnyArg())).
		WillReturnResult(1)

	n, err := mock.CopyFrom(context.Background(), copyTable, columns,
		pgx.CopyFromRows([][]any{{"alice", time.Now()}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCopyFromMatcherMismatch(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("alice", neverMatches{})).
		WillReturnResult(1)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "could not match value 1 of row 0")
}

type neverMatches struct{}

func (neverMatches) Match(any) bool { return false }

// A source with no order of its own is matched as a set.
func TestCopyFromUnordered(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).
			AddRow("alice", 30).
			AddRow("bob", 40).
			Unordered()).
		WillReturnResult(2)

	n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"bob", 40}, {"alice", 30}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCopyFromUnorderedMismatch(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).
			AddRow("alice", 30).
			AddRow("bob", 40).
			Unordered()).
		WillReturnResult(2)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"bob", 40}, {"bob", 40}}))
	assert.ErrorContains(t, err, "matches none of the expected rows left to pair")
}

// A bulk copy stays readable as csv.
func TestCopyFromFromCSVString(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).FromCSVString("alice,30\nbob,40")).
		WillReturnResult(2)

	n, err := mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", "30"}, {"bob", "40"}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Columns carrying an OID are compared through the codec registered for it, so
// a custom type compares here as it would against a server.
func TestCopyFromComparesThroughRegisteredCodec(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	const statusOID = 100000
	mock.TypeMap().RegisterType(&pgtype.Type{
		Name: "status", OID: statusOID, Codec: pgtype.TextCodec{},
	})

	column := pgconn.FieldDescription{Name: "status", DataTypeOID: statusOID}
	mock.ExpectCopyFrom(copyTable, []string{"status"}).
		WithRows(NewCopyRowsWithColumnDefinition(column).AddRow("active")).
		WillReturnResult(1)

	n, err := mock.CopyFrom(context.Background(), copyTable, []string{"status"},
		pgx.CopyFromRows([][]any{{[]byte("active")}}))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

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
		WithRows(NewCopyRows(copyColumns...)).
		WillReturnResult(0)

	_, err = mock.CopyFrom(context.Background(), copyTable, copyColumns,
		pgx.CopyFromRows([][]any{{"alice", 30}}))
	assert.ErrorContains(t, err, "expected 0 row(s) to be copied, but got 1")
}

func TestCopyFromStringIncludesRows(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)
	e := mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("alice", 30))

	assert.Contains(t, e.String(), "matches 1 row(s)")
	assert.Contains(t, e.String(), "alice")

	e2 := mock.ExpectCopyFrom(copyTable, copyColumns).
		WithRows(NewCopyRows(copyColumns...).AddRow("bob", 40).Unordered())
	assert.Contains(t, e2.String(), "matches 1 row(s) in any order")
}
