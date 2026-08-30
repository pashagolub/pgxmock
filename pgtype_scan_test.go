package pgxmock

import (
	"context"
	"net/netip"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func columnOfType(name string, oid uint32) pgconn.FieldDescription {
	return pgconn.FieldDescription{Name: name, DataTypeOID: oid}
}

// A column that declares its OID is decoded through pgtype, so conversions
// plain reflection cannot express start working.
func TestScanArrayThroughTypeMap(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	rows := NewRowsWithColumnDefinition(columnOfType("ids", pgtype.Int4ArrayOID)).
		AddRow([]int32{1, 2, 3})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT ids FROM t")
	assert.NoError(t, err)
	defer rs.Close()
	assert.True(t, rs.Next())

	var ids []int64 // note: not []int32
	assert.NoError(t, rs.Scan(&ids))
	assert.Equal(t, []int64{1, 2, 3}, ids)
}

func TestScanInetThroughTypeMap(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	rows := NewRowsWithColumnDefinition(columnOfType("addr", pgtype.InetOID)).
		AddRow(netip.MustParsePrefix("192.168.1.0/24"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT addr FROM t")
	assert.NoError(t, err)
	defer rs.Close()
	assert.True(t, rs.Next())

	var addr netip.Prefix
	assert.NoError(t, rs.Scan(&addr))
	assert.Equal(t, "192.168.1.0/24", addr.String())
}

// A type registered on the mock's map is honoured, which is the point of
// exposing TypeMap().
func TestScanRegisteredTypeThroughTypeMap(t *testing.T) {
	const statusOID = 90001

	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.TypeMap().RegisterType(&pgtype.Type{
		Name:  "status",
		OID:   statusOID,
		Codec: &pgtype.EnumCodec{},
	})

	rows := NewRowsWithColumnDefinition(columnOfType("status", statusOID)).
		AddRow("active")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT status FROM t")
	assert.NoError(t, err)
	defer rs.Close()
	assert.True(t, rs.Next())

	var status pgtype.Text
	assert.NoError(t, rs.Scan(&status))
	assert.True(t, status.Valid)
	assert.Equal(t, "active", status.String)
}

// Columns without an OID still report the original, descriptive error rather
// than a pgtype one.
func TestScanWithoutOIDKeepsOriginalError(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT").
		WillReturnRows(NewRows([]string{"ids"}).AddRow([]int32{1, 2, 3}))

	rs, err := mock.Query(context.Background(), "SELECT ids FROM t")
	assert.NoError(t, err)
	defer rs.Close()
	assert.True(t, rs.Next())

	var ids []int64
	assert.ErrorContains(t, rs.Scan(&ids), "not supported for value kind")
}

// A value the codec cannot handle reports the codec's error, not a generic one.
func TestScanTypeMapEncodeFailure(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	rows := NewRowsWithColumnDefinition(columnOfType("ids", pgtype.Int4ArrayOID)).
		AddRow(struct{ Nope bool }{})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT ids FROM t")
	assert.NoError(t, err)
	defer rs.Close()
	assert.True(t, rs.Next())

	var ids []int64
	assert.Error(t, rs.Scan(&ids))
}

func TestTypeMapIsPerMock(t *testing.T) {
	first, err := NewConn()
	assert.NoError(t, err)
	second, err := NewConn()
	assert.NoError(t, err)

	assert.NotNil(t, first.TypeMap())
	assert.NotSame(t, first.TypeMap(), second.TypeMap(),
		"each mock must own its type map so registrations do not leak between tests")
}
