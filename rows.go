package pgxmock

import (
	"encoding/csv"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
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

type rowSets struct {
	sets []*Rows
	pos  int
	ex   *ExpectedQuery
}

func (rs *rowSets) Err() error {
	r := rs.sets[rs.pos]
	return r.nextErr[r.pos-1]
}

func (rs *rowSets) CommandTag() pgconn.CommandTag {
	return rs.sets[rs.pos].commandTag
}

func (rs *rowSets) FieldDescriptions() []pgproto3.FieldDescription {
	return rs.sets[rs.pos].defs
}

// func (rs *rowSets) Columns() []string {
// 	return rs.sets[rs.pos].cols
// }

func (rs *rowSets) Close() {
	rs.ex.rowsWereClosed = true
	// return rs.sets[rs.pos].closeErr
}

// advances to next row
func (rs *rowSets) Next() bool {
	r := rs.sets[rs.pos]
	r.pos++
	return r.pos <= len(r.rows)
}

// Values returns the decoded row values. As with Scan(), it is an error to
// call Values without first calling Next() and checking that it returned
// true.
func (rs *rowSets) Values() ([]interface{}, error) {
	r := rs.sets[rs.pos]
	return r.rows[r.pos-1], r.nextErr[r.pos-1]
}

func (rs *rowSets) Scan(dest ...interface{}) error {
	r := rs.sets[rs.pos]
	if len(dest) != len(r.defs) {
		return fmt.Errorf("Incorrect argument number %d for columns %d", len(dest), len(r.defs))
	}
	if len(r.rows) == 0 {
		return pgx.ErrNoRows
	}
	for i, col := range r.rows[r.pos-1] {
		if dest[i] == nil {
			//behave compatible with pgx
			continue
		}
		destVal := reflect.ValueOf(dest[i])
		if destVal.Kind() != reflect.Ptr {
			return fmt.Errorf("Destination argument must be a pointer for column %s", r.defs[i].Name)
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
				return fmt.Errorf("Cannot set destination  value for column %s", string(r.defs[i].Name))
			}
		} else {
			err := convertAssign(dest[i], col)
			if err != nil {
				return err
			}
		}
	}
	return r.nextErr[r.pos-1]
}

func (rs *rowSets) RawValues() [][]byte {
	r := rs.sets[rs.pos]
	dest := make([][]byte, len(r.defs))

	for i, col := range r.rows[r.pos-1] {
		if b, ok := rawBytes(col); ok {
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
	b := make([]byte, len(val))
	copy(b, val)
	return b, true
}

// Rows is a mocked collection of rows to
// return for Query result
type Rows struct {
	commandTag pgconn.CommandTag
	defs       []pgproto3.FieldDescription
	rows       [][]interface{}
	pos        int
	nextErr    map[int]error
	closeErr   error
}

// NewRows allows Rows to be created from a
// sql interface{} slice or from the CSV string and
// to be used as sql driver.Rows.
// Use pgxmock.NewRows instead if using a custom converter
func NewRows(columns []string) *Rows {
	var coldefs []pgproto3.FieldDescription
	for _, column := range columns {
		coldefs = append(coldefs, pgproto3.FieldDescription{Name: []byte(column)})
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
func (r *Rows) AddRow(values ...interface{}) *Rows {
	if len(values) != len(r.defs) {
		panic("Expected number of values to match number of columns")
	}

	row := make([]interface{}, len(r.defs))
	copy(row, values)
	r.rows = append(r.rows, row)
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

// // Implement the "RowsNextResultSet" interface
// func (rs *rowSets) HasNextResultSet() bool {
// 	return rs.pos+1 < len(rs.sets)
// }

// // Implement the "RowsNextResultSet" interface
// func (rs *rowSets) NextResultSet() error {
// 	if !rs.HasNextResultSet() {
// 		return io.EOF
// 	}

// 	rs.pos++
// 	return nil
// }

// type for rows with columns definition created with pgxmock.NewRowsWithColumnDefinition
type rowSetsWithDefinition struct {
	*rowSets
}

// NewRowsWithColumnDefinition return rows with columns metadata
func NewRowsWithColumnDefinition(columns ...pgproto3.FieldDescription) *Rows {
	return &Rows{
		defs:    columns,
		nextErr: make(map[int]error),
	}
}

var errNilPtr = errors.New("destination pointer is nil")

type bytes []byte

// convertAssign copies to dest the value in src, converting if possible.
// yoinked from database/sql in Go 1.19.1 along with its helper functions
// and types.
func convertAssign(dest, src interface{}) error {
	// Common cases, without reflect.
	switch s := src.(type) {
	case string:
		switch d := dest.(type) {
		case *string:
			if d == nil {
				return errNilPtr
			}
			*d = s
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = []byte(s)
			return nil
		case *bytes:
			if d == nil {
				return errNilPtr
			}
			*d = append((*d)[:0], s...)
			return nil
		}
	case []byte:
		switch d := dest.(type) {
		case *string:
			if d == nil {
				return errNilPtr
			}
			*d = string(s)
			return nil
		case *interface{}:
			if d == nil {
				return errNilPtr
			}
			*d = cloneBytes(s)
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = cloneBytes(s)
			return nil
		case *bytes:
			if d == nil {
				return errNilPtr
			}
			*d = s
			return nil
		}
	case time.Time:
		switch d := dest.(type) {
		case *time.Time:
			*d = s
			return nil
		case *string:
			*d = s.Format(time.RFC3339Nano)
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = []byte(s.Format(time.RFC3339Nano))
			return nil
		case *bytes:
			if d == nil {
				return errNilPtr
			}
			*d = s.AppendFormat((*d)[:0], time.RFC3339Nano)
			return nil
		}
	case decimalDecompose:
		switch d := dest.(type) {
		case decimalCompose:
			return d.Compose(s.Decompose(nil))
		}
	case nil:
		switch d := dest.(type) {
		case *interface{}:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		case *bytes:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		}
	}

	var sv reflect.Value

	switch d := dest.(type) {
	case *string:
		sv = reflect.ValueOf(src)
		switch sv.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			*d = asString(src)
			return nil
		}
	case *[]byte:
		sv = reflect.ValueOf(src)
		if b, ok := asBytes(nil, sv); ok {
			*d = b
			return nil
		}
	case *bytes:
		sv = reflect.ValueOf(src)
		if b, ok := asBytes([]byte(*d)[:0], sv); ok {
			*d = b
			return nil
		}
	case *bool:
		bv, err := asBool(src)
		if err == nil {
			*d = bv
		}
		return err
	case *interface{}:
		*d = src
		return nil
	}

	if scanner, ok := dest.(interface{ Scan(interface{}) error }); ok {
		return scanner.Scan(src)
	}

	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Pointer {
		return errors.New("destination not a pointer")
	}
	if dpv.IsNil() {
		return errNilPtr
	}

	if !sv.IsValid() {
		sv = reflect.ValueOf(src)
	}

	dv := reflect.Indirect(dpv)
	if sv.IsValid() && sv.Type().AssignableTo(dv.Type()) {
		switch b := src.(type) {
		case []byte:
			dv.Set(reflect.ValueOf(cloneBytes(b)))
		default:
			dv.Set(sv)
		}
		return nil
	}

	if dv.Kind() == sv.Kind() && sv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(sv.Convert(dv.Type()))
		return nil
	}

	// The following conversions use a string value as an intermediate representation
	// to convert between various numeric types.
	//
	// This also allows scanning into user defined types such as "type Int int64".
	// For symmetry, also check for string destination types.
	switch dv.Kind() {
	case reflect.Pointer:
		if src == nil {
			dv.Set(reflect.Zero(dv.Type()))
			return nil
		}
		dv.Set(reflect.New(dv.Type().Elem()))
		return convertAssign(dv.Interface(), src)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := asString(src)
		i64, err := strconv.ParseInt(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetInt(i64)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := asString(src)
		u64, err := strconv.ParseUint(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetUint(u64)
		return nil
	case reflect.Float32, reflect.Float64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := asString(src)
		f64, err := strconv.ParseFloat(s, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetFloat(f64)
		return nil
	case reflect.String:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		switch v := src.(type) {
		case string:
			dv.SetString(v)
			return nil
		case []byte:
			dv.SetString(string(v))
			return nil
		}
	}

	return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, dest)
}

func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func asString(src interface{}) string {
	switch v := src.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool())
	}
	return fmt.Sprintf("%v", src)
}

func asBytes(buf []byte, rv reflect.Value) (b []byte, ok bool) {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(buf, rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.AppendUint(buf, rv.Uint(), 10), true
	case reflect.Float32:
		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 32), true
	case reflect.Float64:
		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 64), true
	case reflect.Bool:
		return strconv.AppendBool(buf, rv.Bool()), true
	case reflect.String:
		s := rv.String()
		return append(buf, s...), true
	}
	return
}

func asBool(src interface{}) (bool, error) {
	switch s := src.(type) {
	case bool:
		return s, nil
	case string:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return false, fmt.Errorf("couldn't convert %q into type bool", s)
		}
		return b, nil
	case []byte:
		b, err := strconv.ParseBool(string(s))
		if err != nil {
			return false, fmt.Errorf("couldn't convert %q into type bool", s)
		}
		return b, nil
	case int, int16, int32, int64, uint, uint16, uint32, uint64:
		b, err := strconv.ParseBool(asString(src))
		if err != nil {
			return false, fmt.Errorf("couldn't convert %v (%T) into type bool", src, src)
		}
		return b, nil
	default:
		return false, fmt.Errorf("couldn't convert %v (%T) into type bool", src, src)
	}
}

type decimalDecompose interface {
	// Decompose returns the internal decimal state in parts.
	// If the provided buf has sufficient capacity, buf may be returned as the coefficient with
	// the value set and length set as appropriate.
	Decompose(buf []byte) (form byte, negative bool, coefficient []byte, exponent int32)
}

type decimalCompose interface {
	// Compose sets the internal decimal value from parts. If the value cannot be
	// represented then an error should be returned.
	Compose(form byte, negative bool, coefficient []byte, exponent int32) error
}
