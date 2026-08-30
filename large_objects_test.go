package pgxmock

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// If pgx ever stops holding a pgx.Tx in LargeObjects, LargeObjects() can no
// longer wire itself to the mock. Fail here, in this repository's CI, rather
// than in a user's test run.
func TestLargeObjectsLayoutAssumption(t *testing.T) {
	assert.GreaterOrEqual(t, largeObjectsTxIndex, 0,
		"pgx.LargeObjects must expose a pgx.Tx field for the mock to bind to")

	loType := reflect.TypeOf(pgx.LargeObjects{})
	assert.Equal(t, reflect.TypeOf((*pgx.Tx)(nil)).Elem(),
		loType.Field(largeObjectsTxIndex).Type)
}

func TestLargeObjectCreate(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectQuery("select lo_create($1)").
		WithArgs(uint32(0)).
		WillReturnRows(NewRows([]string{"lo_create"}).AddRow(uint32(42)))

	lo := mock.LargeObjects()
	oid, err := lo.Create(context.Background(), 0)
	assert.NoError(t, err)
	assert.Equal(t, uint32(42), oid)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLargeObjectUnlink(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectQuery("select lo_unlink($1)").
		WithArgs(uint32(42)).
		WillReturnRows(NewRows([]string{"lo_unlink"}).AddRow(int32(1)))

	lo := mock.LargeObjects()
	assert.NoError(t, lo.Unlink(context.Background(), 42))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A failed unlink is reported by pgx as an error, driven entirely by the
// mocked result.
func TestLargeObjectUnlinkFailure(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectQuery("select lo_unlink($1)").
		WithArgs(uint32(42)).
		WillReturnRows(NewRows([]string{"lo_unlink"}).AddRow(int32(0)))

	lo := mock.LargeObjects()
	assert.ErrorContains(t, lo.Unlink(context.Background(), 42), "failed to remove large object")
}

// Open returns a pgx.LargeObject that keeps issuing its reads and writes
// through the mock.
func TestLargeObjectReadWrite(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectQuery("select lo_open($1, $2)").
		WithArgs(uint32(42), pgx.LargeObjectModeRead|pgx.LargeObjectModeWrite).
		WillReturnRows(NewRows([]string{"lo_open"}).AddRow(int32(7)))
	mock.ExpectQuery("select lowrite($1, $2)").
		WithArgs(int32(7), []byte("hello")).
		WillReturnRows(NewRows([]string{"lowrite"}).AddRow(int32(5)))
	mock.ExpectQuery("select loread($1, $2)").
		WithArgs(int32(7), 5).
		WillReturnRows(NewRows([]string{"loread"}).AddRow([]byte("hello")))
	mock.ExpectExec("select lo_close($1)").
		WithArgs(int32(7)).
		WillReturnResult(NewResult("SELECT", 1))

	lo := mock.LargeObjects()
	obj, err := lo.Open(context.Background(), 42,
		pgx.LargeObjectModeRead|pgx.LargeObjectModeWrite)
	assert.NoError(t, err)

	n, err := obj.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	buf := make([]byte, 5)
	n, err = obj.Read(buf)
	if err != nil {
		assert.ErrorIs(t, err, io.EOF)
	}
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), buf)

	assert.NoError(t, obj.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Large objects are only valid inside a transaction, and the mocked
// transaction is the mock itself.
func TestLargeObjectsWithinTransaction(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery("select lo_create($1)").
		WithArgs(uint32(0)).
		WillReturnRows(NewRows([]string{"lo_create"}).AddRow(uint32(42)))
	mock.ExpectCommit()

	tx, err := mock.Begin(context.Background())
	assert.NoError(t, err)

	lo := tx.LargeObjects()
	oid, err := lo.Create(context.Background(), 0)
	assert.NoError(t, err)
	assert.Equal(t, uint32(42), oid)

	assert.NoError(t, tx.Commit(context.Background()))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLargeObjectSeekTellTruncate(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectQuery("select lo_open($1, $2)").
		WithArgs(uint32(42), pgx.LargeObjectModeWrite).
		WillReturnRows(NewRows([]string{"lo_open"}).AddRow(int32(7)))
	mock.ExpectQuery("select lo_lseek64($1, $2, $3)").
		WithArgs(int32(7), int64(10), 0).
		WillReturnRows(NewRows([]string{"lo_lseek64"}).AddRow(int64(10)))
	mock.ExpectQuery("select lo_tell64($1)").
		WithArgs(int32(7)).
		WillReturnRows(NewRows([]string{"lo_tell64"}).AddRow(int64(10)))
	mock.ExpectExec("select lo_truncate64($1, $2)").
		WithArgs(int32(7), int64(4)).
		WillReturnResult(NewResult("SELECT", 1))

	lo := mock.LargeObjects()
	obj, err := lo.Open(context.Background(), 42, pgx.LargeObjectModeWrite)
	assert.NoError(t, err)

	pos, err := obj.Seek(10, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), pos)

	pos, err = obj.Tell()
	assert.NoError(t, err)
	assert.Equal(t, int64(10), pos)

	assert.NoError(t, obj.Truncate(4))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Errors arranged on the expectation surface through the large object API.
func TestLargeObjectCreateError(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectQuery("lo_create").WithArgs(uint32(0)).WillReturnError(errLargeObject)

	lo := mock.LargeObjects()
	_, err = lo.Create(context.Background(), 0)
	assert.ErrorIs(t, err, errLargeObject)
}

var errLargeObject = errors.New("large object failure")
