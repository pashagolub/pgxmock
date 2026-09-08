package pgxmock

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const statusTypeOID = 90001

func statusType() *pgtype.Type {
	return &pgtype.Type{Name: "status", OID: statusTypeOID, Codec: &pgtype.EnumCodec{}}
}

// The loaded type can be registered and then scanned.
func TestLoadTypesThenRegisterAndScan(t *testing.T) {
	t.Parallel()
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	require.NoError(t, err)

	mock.ExpectLoadTypes("status").WillReturnTypes(statusType())
	mock.ExpectQuery("SELECT").WillReturnRows(
		NewRowsWithColumnDefinition(columnOfType("status", statusTypeOID)).AddRow("active"))

	types, err := mock.LoadTypes(context.Background(), []string{"status"})
	require.NoError(t, err)
	require.Len(t, types, 1)
	assert.Equal(t, "status", types[0].Name)

	// as against a real connection, registering is the caller's job
	mock.TypeMap().RegisterTypes(types)

	rows, err := mock.Query(context.Background(), "SELECT status FROM t")
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())

	var status pgtype.Text
	require.NoError(t, rows.Scan(&status))
	assert.Equal(t, "active", status.String)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadType(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	mock.ExpectLoadType("status").WillReturnTypes(statusType())

	typ, err := mock.LoadType(context.Background(), "status")
	require.NoError(t, err)
	assert.Equal(t, uint32(statusTypeOID), typ.OID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadTypesUnexpectedNames(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	mock.ExpectLoadTypes("status", "address").WillReturnTypes(statusType())

	_, err = mock.LoadTypes(context.Background(), []string{"status"})
	assert.ErrorContains(t, err, "were not expected")
}

func TestLoadTypeDoesNotMatchLoadTypes(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	mock.ExpectLoadTypes("status").WillReturnTypes(statusType())

	_, err = mock.LoadType(context.Background(), "status")
	assert.Error(t, err)
}

func TestLoadTypesWillReturnError(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	want := errors.New("type does not exist")
	mock.ExpectLoadTypes("status").WillReturnError(want)

	types, err := mock.LoadTypes(context.Background(), []string{"status"})
	assert.ErrorIs(t, err, want)
	assert.Nil(t, types)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadTypesWithoutResult(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	mock.ExpectLoadTypes("status")

	_, err = mock.LoadTypes(context.Background(), []string{"status"})
	assert.ErrorContains(t, err, "must return types or raise an error")
}

// Like pgx, an empty list fails before reaching an expectation.
func TestLoadTypesWithoutNames(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	require.NoError(t, err)

	mock.ExpectLoadTypes("status").WillReturnTypes(statusType())

	_, err = mock.LoadTypes(context.Background(), nil)
	assert.ErrorContains(t, err, "No type names were supplied.")
	assert.ErrorContains(t, mock.ExpectationsWereMet(), "remaining expectation")
}

func TestLoadTypesOnClosedConn(t *testing.T) {
	t.Parallel()
	mock, err := NewConn(ErrorOnClosedConnOption())
	require.NoError(t, err)
	mock.ExpectClose()
	mock.ExpectLoadTypes("status").WillReturnTypes(statusType()).Maybe()

	require.NoError(t, mock.Close(context.Background()))

	_, err = mock.LoadTypes(context.Background(), []string{"status"})
	assert.ErrorIs(t, err, pgconn.ErrConnClosed)
}

func TestLoadTypesThroughPool(t *testing.T) {
	t.Parallel()
	mock, err := NewPool()
	require.NoError(t, err)

	mock.ExpectLoadTypes("status").WillReturnTypes(statusType())

	types, err := mock.AsConn().LoadTypes(context.Background(), []string{"status"})
	require.NoError(t, err)
	assert.Len(t, types, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}
