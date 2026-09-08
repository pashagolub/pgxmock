package pgxmock

import (
	"github.com/jackc/pgx/v5/pgconn"
)

// CopyRows is the set of rows a CopyFrom is expected to copy, assembled the way
// a query result is assembled with NewRows.
//
// It is a type of its own rather than a Rows because a copy expectation is an
// input: the parts of Rows that describe a result - RowError, CloseError,
// AddCommandTag, Kind - say nothing about data being sent to a server, and are
// therefore not reachable here. The row storage, the arity check and the CSV
// parsing are shared.
type CopyRows struct {
	rows      Rows
	unordered bool
}

// NewCopyRows creates the rows a CopyFrom is expected to copy over the given
// columns. The names are matched against the columns passed to ExpectCopyFrom,
// so rows assembled in a different order are reported instead of being compared
// position by position against the wrong column.
func NewCopyRows(columns ...string) *CopyRows {
	return &CopyRows{rows: *NewRows(columns)}
}

// NewCopyRowsWithColumnDefinition creates expected copy rows whose columns carry
// pgtype metadata. A column with a DataTypeOID is compared through the codec
// registered for it in pgxmock.TypeMap, which is what makes a custom type
// compare here the way it would against a server.
func NewCopyRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *CopyRows {
	return &CopyRows{rows: *NewRowsWithColumnDefinition(columns...)}
}

// AddRow adds a row the pgx.CopyFromSource is expected to yield. The number of
// values must match the number of columns; that is checked here rather than
// when CopyFrom runs, so a miscounted row is reported at the line that wrote it.
//
// A value may be an Argument matcher, such as AnyArg, to stand in for anything
// the test cannot predict.
func (cr *CopyRows) AddRow(values ...any) *CopyRows {
	cr.rows.AddRow(values...)
	return cr
}

// AddRows adds several expected rows at once.
func (cr *CopyRows) AddRows(values ...[]any) *CopyRows {
	cr.rows.AddRows(values...)
	return cr
}

// FromCSVString builds the expected rows from a csv string, which keeps a bulk
// copy readable. Values are parsed by CSVColumnParser, so they arrive as
// strings and are compared through the column codec, if the column has one.
func (cr *CopyRows) FromCSVString(s string) *CopyRows {
	cr.rows.FromCSVString(s)
	return cr
}

// Unordered matches the copied rows as a set rather than a sequence, for a
// source that iterates something with no order of its own, such as a map.
//
// Rows are paired greedily in the order they were copied, so expectations whose
// Argument matchers overlap may pair differently than a reader expects; state
// those in order instead.
func (cr *CopyRows) Unordered() *CopyRows {
	cr.unordered = true
	return cr
}

// columnNames returns the names the rows were declared with.
func (cr *CopyRows) columnNames() []string {
	names := make([]string, len(cr.rows.defs))
	for i, def := range cr.rows.defs {
		names[i] = def.Name
	}
	return names
}

// oidOf returns the OID column j was declared with, or 0 when the column
// carries no type information and the value has to speak for itself.
func (cr *CopyRows) oidOf(j int) uint32 {
	if j < len(cr.rows.defs) {
		return cr.rows.defs[j].DataTypeOID
	}
	return 0
}
