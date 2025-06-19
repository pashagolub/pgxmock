package pgxmock

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// CSVColumnParser is a function which converts trimmed csv
// column string to a []byte representation. Currently
// transforms NULL to nil
var CSVColumnParser = func(s string) interface{} {
	switch {
	case strings.ToLower(s) == "null":
		return nil
	}
	return s
}

// connRow implements the Row interface for Conn.QueryRow.
type connRow rowSets

func (r *connRow) Scan(dest ...any) (err error) {
	rows := (*rowSets)(r)

	if rows.Err() != nil {
		return rows.Err()
	}

	for _, d := range dest {
		if _, ok := d.(*pgtype.DriverBytes); ok {
			rows.Close()
			return fmt.Errorf("cannot scan into *pgtype.DriverBytes from QueryRow")
		}
	}

	if !rows.Next() {
		if rows.Err() == nil {
			return pgx.ErrNoRows
		}
		return rows.Err()
	}
	defer rows.Close()
	return errors.Join(rows.Scan(dest...), rows.Err())
}

type rowSets struct {
	sets     []*Rows
	RowSetNo int
	ex       *ExpectedQuery
}

func (rs *rowSets) Conn() *pgx.Conn {
	return nil
}

func (rs *rowSets) Err() error {
	r := rs.sets[rs.RowSetNo]
	return r.nextErr[r.recNo-1]
}

func (rs *rowSets) CommandTag() pgconn.CommandTag {
	return rs.sets[rs.RowSetNo].commandTag
}

func (rs *rowSets) FieldDescriptions() []pgconn.FieldDescription {
	return rs.sets[rs.RowSetNo].defs
}

// func (rs *rowSets) Columns() []string {
// 	return rs.sets[rs.pos].cols
// }

func (rs *rowSets) Close() {
	if rs.ex != nil {
		rs.ex.rowsWereClosed = true
	}
	// return rs.sets[rs.pos].closeErr
}

// advances to next row
func (rs *rowSets) Next() bool {
	r := rs.sets[rs.RowSetNo]
	r.recNo++
	return r.recNo <= len(r.rows)
}

// Values returns the decoded row values. As with Scan(), it is an error to
// call Values without first calling Next() and checking that it returned
// true.
func (rs *rowSets) Values() ([]interface{}, error) {
	r := rs.sets[rs.RowSetNo]
	return r.rows[r.recNo-1], r.nextErr[r.recNo-1]
}

func (rs *rowSets) Scan(dest ...interface{}) error {
	r := rs.sets[rs.RowSetNo]
	if len(dest) == 1 {
		if rc, ok := dest[0].(pgx.RowScanner); ok {
			return rc.ScanRow(rs)
		}
	}
	if len(dest) != len(r.defs) {
		return fmt.Errorf("incorrect argument number %d for columns %d", len(dest), len(r.defs))
	}
	if len(r.rows) == 0 {
		return pgx.ErrNoRows
	}
	for i, col := range r.rows[r.recNo-1] {
		if dest[i] == nil {
			//behave compatible with pgx
			continue
		}
		destVal := reflect.ValueOf(dest[i])
		if destVal.Kind() != reflect.Ptr {
			return fmt.Errorf("destination argument must be a pointer for column %s", r.defs[i].Name)
		}
		if col == nil {
			dest[i] = nil
			continue
		}
		val := reflect.ValueOf(col)
		if _, ok := dest[i].(*interface{}); ok || val.Type().AssignableTo(destVal.Elem().Type()) {
			if destElem := destVal.Elem(); destElem.CanSet() {
				destElem.Set(val)
			} else {
				return fmt.Errorf("cannot set destination value for column %s", r.defs[i].Name)
			}
		} else if scanner, ok := destVal.Interface().(interface{ Scan(interface{}) error }); ok {
			// Try to use Scanner interface
			if err := scanner.Scan(val.Interface()); err != nil {
				return fmt.Errorf("scanning value error for column '%s': %w", string(r.defs[i].Name), err)
			}
		} else if val.CanConvert(destVal.Elem().Type()) {
			if destElem := destVal.Elem(); destElem.CanSet() {
				destElem.Set(val.Convert(destElem.Type()))
			} else {
				return fmt.Errorf("cannot set destination value for column %s", r.defs[i].Name)
			}
		} else {
			return fmt.Errorf("destination kind '%v' not supported for value kind '%v' of column '%s'",
				destVal.Elem().Kind(), val.Kind(), string(r.defs[i].Name))
		}
	}
	return r.nextErr[r.recNo-1]
}

func (rs *rowSets) RawValues() [][]byte {
	r := rs.sets[rs.RowSetNo]
	dest := make([][]byte, len(r.defs))

	for i, col := range r.rows[r.recNo-1] {
		if b, ok := rawBytes(col); ok {
			dest[i] = b
			continue
		}
		dest[i] = []byte(fmt.Sprintf("%v", col))
	}

	return dest
}

// transforms to debuggable printable string
func (rs *rowSets) String() string {
	if rs.empty() {
		return "\t- returns no data"
	}

	msg := "\t- returns data:\n"
	if len(rs.sets) == 1 {
		for n, row := range rs.sets[0].rows {
			msg += fmt.Sprintf("\t\trow %d - %+v\n", n, row)
		}
		return msg
	}
	for i, set := range rs.sets {
		msg += fmt.Sprintf("\t\tresult set: %d\n", i)
		for n, row := range set.rows {
			msg += fmt.Sprintf("\t\t\trow %d: %+v\n", n, row)
		}
	}
	return msg
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
	val, err := json.Marshal(col)
	if err != nil || len(val) == 0 {
		return nil, false
	}
	// Copy the bytes from the mocked row into a shared raw buffer, which we'll replace the content of later
	b := make([]byte, len(val))
	copy(b, val)
	return b, true
}

// Rows is a mocked collection of rows to
// return for Query result
type Rows struct {
	commandTag pgconn.CommandTag
	defs       []pgconn.FieldDescription
	rows       [][]interface{}
	recNo      int
	nextErr    map[int]error
	closeErr   error
}

// NewRows allows Rows to be created from a
// sql interface{} slice or from the CSV string and
// to be used as sql driver.Rows.
// Use pgxmock.NewRows instead if using a custom converter
func NewRows(columns []string) *Rows {
	var coldefs []pgconn.FieldDescription
	for _, column := range columns {
		coldefs = append(coldefs, pgconn.FieldDescription{Name: column})
	}
	return &Rows{
		defs:    coldefs,
		nextErr: make(map[int]error),
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
func (r *Rows) AddRow(values ...any) *Rows {
	if len(values) != len(r.defs) {
		panic("Expected number of values to match number of columns")
	}

	row := make([]interface{}, len(r.defs))
	copy(row, values)
	r.rows = append(r.rows, row)
	return r
}

// AddRows adds multiple rows composed from any slice and
// returns the same instance to perform subsequent actions.
func (r *Rows) AddRows(values ...[]any) *Rows {
	for _, value := range values {
		r.AddRow(value...)
	}
	return r
}

// AddCommandTag will add a command tag to the result set
func (r *Rows) AddCommandTag(tag pgconn.CommandTag) *Rows {
	r.commandTag = tag
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

		row := make([]interface{}, len(r.defs))
		for i, v := range res {
			row[i] = CSVColumnParser(strings.TrimSpace(v))
		}
		r.rows = append(r.rows, row)
	}
	return r
}

// Kind returns rows corresponding to the interface pgx.Rows
// useful for testing entities that implement an interface pgx.RowScanner
func (r *Rows) Kind() pgx.Rows {
	return &rowSets{
		sets: []*Rows{r},
	}
}

// NewRowsWithColumnDefinition return rows with columns metadata
func NewRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *Rows {
	return &Rows{
		defs:    columns,
		nextErr: make(map[int]error),
	}
}
