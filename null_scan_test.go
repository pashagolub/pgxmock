package pgxmock

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func scanNullInto(t *testing.T, dest ...any) error {
	t.Helper()
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"col"}).AddRow(nil))
	rows, err := mock.Query(context.Background(), "SELECT col FROM t")
	assert.NoError(t, err)
	defer rows.Close()
	assert.True(t, rows.Next())
	return rows.Scan(dest...)
}

// A NULL must actually reach the destination instead of being dropped, which
// used to leave whatever the variable held before the scan.
func TestScanNullClearsNillableDestinations(t *testing.T) {
	preset := "preexisting"

	strPtr := &preset
	assert.NoError(t, scanNullInto(t, &strPtr))
	assert.Nil(t, strPtr, "*string destination must be set to nil")

	anyDest := any("preexisting")
	assert.NoError(t, scanNullInto(t, &anyDest))
	assert.Nil(t, anyDest, "any destination must be set to nil")

	bytesDest := []byte("preexisting")
	assert.NoError(t, scanNullInto(t, &bytesDest))
	assert.Nil(t, bytesDest, "[]byte destination must be set to nil")

	mapDest := map[string]string{"k": "v"}
	assert.NoError(t, scanNullInto(t, &mapDest))
	assert.Nil(t, mapDest, "map destination must be set to nil")
}

// pgtype values implement sql.Scanner and must be marked invalid, not left
// untouched.
func TestScanNullIntoScanner(t *testing.T) {
	text := pgtype.Text{String: "preexisting", Valid: true}
	assert.NoError(t, scanNullInto(t, &text))
	assert.False(t, text.Valid, "pgtype.Text must be marked invalid")
	assert.Empty(t, text.String)

	num := pgtype.Int8{Int64: 42, Valid: true}
	assert.NoError(t, scanNullInto(t, &num))
	assert.False(t, num.Valid, "pgtype.Int8 must be marked invalid")
}

// Destinations that cannot represent NULL must report an error, the way pgx
// does, rather than silently keeping their previous value.
func TestScanNullIntoNonNillableDestinationFails(t *testing.T) {
	str := "preexisting"
	err := scanNullInto(t, &str)
	assert.ErrorContains(t, err, "cannot scan NULL into *string")
	assert.ErrorContains(t, err, "col")
	assert.Equal(t, "preexisting", str, "a failed scan must not corrupt the destination")

	n := 42
	assert.ErrorContains(t, scanNullInto(t, &n), "cannot scan NULL into *int")

	var b bool
	assert.ErrorContains(t, scanNullInto(t, &b), "cannot scan NULL into *bool")
}

// Passing a nil destination still skips the column, as pgx allows.
func TestScanNullIntoNilDestinationIsSkipped(t *testing.T) {
	assert.NoError(t, scanNullInto(t, nil))
}
