package pgxmock

import (
	"context"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pgx v5.11 changed name based row to struct mapping: matching became case
// insensitive, with an exact match still winning, and pointer indirection
// deeper than one level started working. Nothing in pgxmock has to do anything
// about that - the mapping runs on top of the pgx.Rows the mock returns - which
// is exactly why it is worth pinning down here: the mock's FieldDescriptions
// and its scanning path are what feed the mapping.

func TestRowToStructByNameIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	type person struct {
		ID   int32
		Name string
	}

	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	require.NoError(t, err)

	mock.ExpectQuery("SELECT").WillReturnRows(
		mock.NewRows([]string{"ID", "NAME"}).AddRow(int32(1), "john"))

	rows, err := mock.Query(context.Background(), "SELECT id, name FROM users")
	require.NoError(t, err)

	got, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[person])
	require.NoError(t, err)
	assert.Equal(t, person{ID: 1, Name: "john"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRowToStructByNamePrefersExactMatch(t *testing.T) {
	t.Parallel()
	type record struct {
		Lower  int32  `db:"foo"`
		Upper  int32  `db:"FOO"`
		Region string `db:"Region"`
	}

	// the column order must not decide which field an exactly matching column
	// binds to, so both orders are checked
	for name, columns := range map[string][]string{
		"lower first": {"foo", "FOO", "region"},
		"upper first": {"FOO", "foo", "region"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
			require.NoError(t, err)

			row := mock.NewRows(columns)
			if columns[0] == "foo" {
				row.AddRow(int32(1), int32(2), "east")
			} else {
				row.AddRow(int32(2), int32(1), "east")
			}
			mock.ExpectQuery("SELECT").WillReturnRows(row)

			rows, err := mock.Query(context.Background(), "SELECT foo, FOO, region FROM t")
			require.NoError(t, err)

			got, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[record])
			require.NoError(t, err)
			assert.Equal(t, record{Lower: 1, Upper: 2, Region: "east"}, got)
		})
	}
}

func TestRowToStructByNameDoublePointer(t *testing.T) {
	t.Parallel()
	type person struct {
		Last  string
		First **string
	}

	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	require.NoError(t, err)

	mock.ExpectQuery("SELECT").WillReturnRows(
		mock.NewRows([]string{"last", "first"}).
			AddRow("Smith", "John").
			AddRow("Smith", nil))

	rows, err := mock.Query(context.Background(), "SELECT last, first FROM people")
	require.NoError(t, err)

	people, err := pgx.CollectRows(rows, pgx.RowToStructByName[person])
	require.NoError(t, err)
	require.Len(t, people, 2)

	require.NotNil(t, people[0].First)
	assert.Equal(t, "John", **people[0].First)
	assert.Nil(t, people[1].First)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The same indirection has to work for a plain Scan, with and without a column
// OID: with one the value is decoded by pgtype, without one by reflection, and
// the two must agree.
func TestScanIntoTriplePointer(t *testing.T) {
	t.Parallel()
	for name, col := range map[string]pgconn.FieldDescription{
		"with OID":    columnOfType("name", pgtype.TextOID),
		"without OID": {Name: "name"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
			require.NoError(t, err)

			mock.ExpectQuery("SELECT").WillReturnRows(
				NewRowsWithColumnDefinition(col).AddRow("john").AddRow(nil))

			rows, err := mock.Query(context.Background(), "SELECT name FROM users")
			require.NoError(t, err)
			defer rows.Close()

			require.True(t, rows.Next())
			var name ***string
			require.NoError(t, rows.Scan(&name))
			require.NotNil(t, name)
			assert.Equal(t, "john", ***name)

			require.True(t, rows.Next())
			require.NoError(t, rows.Scan(&name))
			assert.Nil(t, name, "a NULL must clear the whole chain")
		})
	}
}
