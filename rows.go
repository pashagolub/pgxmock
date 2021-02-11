package pgxmock

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
)

const invalidate = "☠☠☠ MEMORY OVERWRITTEN ☠☠☠ "

// CSVColumnParser is a function which converts trimmed csv
// column string to a []byte representation. Currently
// transforms NULL to nil
var CSVColumnParser = func(s string) []byte {
	switch {
	case strings.ToLower(s) == "null":
		return nil
	}
	return []byte(s)
}

type rowSets struct {
	sets []*Rows
	pos  int
	ex   *ExpectedQuery
	raw  [][]byte
}

func (rs *rowSets) Err() error {
	return rs.sets[rs.pos].closeErr
}

func (rs *rowSets) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag("")
}

func (rs *rowSets) FieldDescriptions() []pgproto3.FieldDescription {
	return rs.sets[rs.pos].def
}

func (rs *rowSets) Columns() []string {
	return rs.sets[rs.pos].cols
}

func (rs *rowSets) Close() {
	rs.invalidateRaw()
	rs.ex.rowsWereClosed = true
	// return rs.sets[rs.pos].closeErr
}

// advances to next row
func (rs *rowSets) Next() bool {
	r := rs.sets[rs.pos]
	r.pos++
	rs.invalidateRaw()
	return r.pos <= len(r.rows)
}

func (rs *rowSets) Values() ([]interface{}, error) {
	return nil, nil
}

func (rs *rowSets) Scan(dest ...interface{}) error {
	r := rs.sets[rs.pos]
	if len(dest) != len(r.cols) {
		return fmt.Errorf("Incorrect argument number %d for columns %d", len(dest), len(r.cols))
	}
	for i, col := range r.rows[r.pos-1] {
		destVal := reflect.ValueOf(dest[i])
		val := reflect.ValueOf(col)
		if destVal.Kind() == reflect.Ptr && destVal.Elem().Kind() == val.Kind() {
			if destElem := destVal.Elem(); destElem.CanSet() {
				destElem.Set(val)
			} else {
				return fmt.Errorf("Cannot set destination value for column %s", r.cols[i])
			}
		} else {
			return fmt.Errorf("Destination kind not supported for column %s", r.cols[i])
		}
	}
	return r.nextErr[r.pos-1]
}

func (rs *rowSets) RawValues() [][]byte {
	r := rs.sets[rs.pos]
	dest := make([][]byte, len(r.cols))

	for i, col := range r.rows[r.pos-1] {
		if b, ok := rawBytes(col); ok {
			rs.raw = append(rs.raw, b)
			dest[i] = b
			continue
		}
		dest[i] = col.([]byte)
	}

	return dest
}

// transforms to debuggable printable string
func (rs *rowSets) String() string {
	if rs.empty() {
		return "with empty rows"
	}

	msg := "should return rows:\n"
	if len(rs.sets) == 1 {
		for n, row := range rs.sets[0].rows {
			msg += fmt.Sprintf("    row %d - %+v\n", n, row)
		}
		return strings.TrimSpace(msg)
	}
	for i, set := range rs.sets {
		msg += fmt.Sprintf("    result set: %d\n", i)
		for n, row := range set.rows {
			msg += fmt.Sprintf("      row %d - %+v\n", n, row)
		}
	}
	return strings.TrimSpace(msg)
}

func (rs *rowSets) empty() bool {
	for _, set := range rs.sets {
		if len(set.rows) > 0 {
			return false
		}
	}
	return true
}

func rawBytes(col interface{}) (_ []byte, ok bool) {
	val, ok := col.([]byte)
	if !ok || len(val) == 0 {
		return nil, false
	}
	// Copy the bytes from the mocked row into a shared raw buffer, which we'll replace the content of later
	// This allows scanning into sql.RawBytes to correctly become invalid on subsequent calls to Next(), Scan() or Close()
	b := make([]byte, len(val))
	copy(b, val)
	return b, true
}

// Bytes that could have been scanned as sql.RawBytes are only valid until the next call to Next, Scan or Close.
// If those occur, we must replace their content to simulate the shared memory to expose misuse of sql.RawBytes
func (rs *rowSets) invalidateRaw() {
	// Replace the content of slices previously returned
	b := []byte(invalidate)
	for _, r := range rs.raw {
		copy(r, bytes.Repeat(b, len(r)/len(b)+1))
	}
	// Start with new slices for the next scan
	rs.raw = nil
}

// Rows is a mocked collection of rows to
// return for Query result
type Rows struct {
	// converter interface{}Converter
	cols     []string
	def      []pgproto3.FieldDescription
	rows     [][]interface{}
	pos      int
	nextErr  map[int]error
	closeErr error
}

// NewRows allows Rows to be created from a
// sql interface{} slice or from the CSV string and
// to be used as sql driver.Rows.
// Use Sqlmock.NewRows instead if using a custom converter
func NewRows(columns []string) *Rows {
	return &Rows{
		cols:    columns,
		nextErr: make(map[int]error),
		// converter: driver.DefaultParameterConverter,
	}
}

// CloseError allows to set an error
// which will be returned by rows.Close
// function.
//
// The close error will be triggered only in cases
// when rows.Next() EOF was not yet reached, that is
// a default sql library behavior
func (r *Rows) CloseError(err error) *Rows {
	r.closeErr = err
	return r
}

// RowError allows to set an error
// which will be returned when a given
// row number is read
func (r *Rows) RowError(row int, err error) *Rows {
	r.nextErr[row] = err
	return r
}

// AddRow composed from database interface{} slice
// return the same instance to perform subsequent actions.
// Note that the number of values must match the number
// of columns
func (r *Rows) AddRow(values ...interface{}) *Rows {
	if len(values) != len(r.cols) {
		panic("Expected number of values to match number of columns")
	}

	row := make([]interface{}, len(r.cols))
	copy(row, values)
	r.rows = append(r.rows, row)
	return r
}

// FromCSVString build rows from csv string.
// return the same instance to perform subsequent actions.
// Note that the number of values must match the number
// of columns
func (r *Rows) FromCSVString(s string) *Rows {
	res := strings.NewReader(strings.TrimSpace(s))
	csvReader := csv.NewReader(res)

	for {
		res, err := csvReader.Read()
		if err != nil || res == nil {
			break
		}

		row := make([]interface{}, len(r.cols))
		for i, v := range res {
			row[i] = CSVColumnParser(strings.TrimSpace(v))
		}
		r.rows = append(r.rows, row)
	}
	return r
}

// Implement the "RowsNextResultSet" interface
func (rs *rowSets) HasNextResultSet() bool {
	return rs.pos+1 < len(rs.sets)
}

// Implement the "RowsNextResultSet" interface
func (rs *rowSets) NextResultSet() error {
	if !rs.HasNextResultSet() {
		return io.EOF
	}

	rs.pos++
	return nil
}

// type for rows with columns definition created with sqlmock.NewRowsWithColumnDefinition
type rowSetsWithDefinition struct {
	*rowSets
}

// NewRowsWithColumnDefinition return rows with columns metadata
func NewRowsWithColumnDefinition(columns ...pgproto3.FieldDescription) *Rows {
	cols := make([]string, len(columns))
	for i, column := range columns {
		cols[i] = string(column.Name)
	}

	return &Rows{
		cols:    cols,
		def:     columns,
		nextErr: make(map[int]error),
		// converter: driver.DefaultParameterConverter,
	}
}
