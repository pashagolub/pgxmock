package pgxmock

import (
	"encoding/csv"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// CSVColumnParser converts a trimmed csv column string to the value stored in
// the row. It maps the literal NULL, in any case, to nil and passes anything
// else through as a string. Replace it to parse CSV columns differently.
var CSVColumnParser = func(s string) any {
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
	// typeMap is the type map of the mock these rows were returned by. It is
	// nil for rows built outside a query, e.g. through Rows.Kind().
	typeMap *lockedTypeMap
}

// getTypeMap returns the mock's type map, falling back to a shared default for
// rows that were not produced by a query.
func (rs *rowSets) getTypeMap() *lockedTypeMap {
	if rs.typeMap != nil {
		return rs.typeMap
	}
	return defaultTypeMap
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
		rs.ex.rowsWereClosed.Store(true)
	}
	rs.close()
}

// close marks the current rows closed, jumps to the last row, and sets the
// close error.
func (rs *rowSets) close() {
	r := rs.sets[rs.RowSetNo]
	r.recNo = len(r.rows)
	r.nextErr[r.recNo-1] = r.closeErr
	r.closed = true
}

// advances to next row
func (rs *rowSets) Next() bool {
	r := rs.sets[rs.RowSetNo]
	if r.recNo == len(r.rows) && r.nextErr[r.recNo] == nil {
		rs.close()
		return false
	}
	r.recNo++
	return r.recNo <= len(r.rows)
}

// Values returns the decoded row values. As with Scan(), it is an error to
// call Values without first calling Next() and checking that it returned
// true.
func (rs *rowSets) Values() ([]any, error) {
	r := rs.sets[rs.RowSetNo]
	return r.rows[r.recNo-1], r.nextErr[r.recNo-1]
}

func (rs *rowSets) Scan(dest ...any) error {
	r := rs.sets[rs.RowSetNo]
	if r.closed {
		// If there is no error, we should return one anyway. Weirdly, pgx returns
		// `number of field descriptions must equal number of values, got %d and %d`.
		return r.nextErr[r.recNo-1]
	}
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
		if destVal.Kind() != reflect.Pointer {
			return fmt.Errorf("destination argument must be a pointer for column %s", r.defs[i].Name)
		}
		if col == nil {
			if err := scanNull(destVal, string(r.defs[i].Name)); err != nil {
				return err
			}
			continue
		}
		val := reflect.ValueOf(col)
		if _, ok := dest[i].(*any); ok || val.Type().AssignableTo(destVal.Elem().Type()) {
			if destElem := destVal.Elem(); destElem.CanSet() {
				destElem.Set(val)
			} else {
				return fmt.Errorf("cannot set destination value for column %s", r.defs[i].Name)
			}
		} else if scanner, ok := destVal.Interface().(interface{ Scan(any) error }); ok {
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
		} else if err := rs.scanViaTypeMap(r.defs[i], col, dest[i]); err == nil {
			// a pgtype codec registered for the column's OID took it
		} else if !errors.Is(err, errNoTypeMapping) {
			return fmt.Errorf("scanning value error for column '%s': %w", string(r.defs[i].Name), err)
		} else {
			return fmt.Errorf("destination kind '%v' not supported for value kind '%v' of column '%s'",
				destVal.Elem().Kind(), val.Kind(), string(r.defs[i].Name))
		}
	}
	return r.nextErr[r.recNo-1]
}

// scanNull assigns a SQL NULL to dest, mirroring how pgx treats NULL values:
// an sql.Scanner is handed a nil, destinations that can represent nil are set
// to their zero value, and anything else is rejected rather than left holding
// whatever it happened to contain before the scan.
func scanNull(destVal reflect.Value, column string) error {
	if scanner, ok := destVal.Interface().(interface{ Scan(any) error }); ok {
		return scanner.Scan(nil)
	}
	elem := destVal.Elem()
	switch elem.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice,
		reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if !elem.CanSet() {
			return fmt.Errorf("cannot set destination value for column %s", column)
		}
		elem.Set(reflect.Zero(elem.Type()))
		return nil
	default:
		return fmt.Errorf("cannot scan NULL into %s for column '%s'", destVal.Type(), column)
	}
}

// errNoTypeMapping reports that a column carries no OID to decode it with, so
// the caller should fall back to its own error.
var errNoTypeMapping = errors.New("no pgtype mapping for column")

// lockedTypeMap pairs a pgtype.Map with the lock it needs, since a
// pgtype.Map is not safe for concurrent use. Every mock owns one, so
// registering a type in one test cannot leak into another and parallel tests
// do not serialise against each other.
type lockedTypeMap struct {
	mu sync.Mutex
	m  *pgtype.Map
}

func newLockedTypeMap() *lockedTypeMap {
	return &lockedTypeMap{m: pgtype.NewMap()}
}

// encode serialises value as the wire representation of oid.
func (t *lockedTypeMap) encode(oid uint32, format int16, value any) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.m.Encode(oid, format, value, nil)
}

// scan decodes src into dest using the codec registered for oid.
func (t *lockedTypeMap) scan(oid uint32, format int16, src []byte, dest any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.m.Scan(oid, format, src, dest)
}

// scanViaTypeMap decodes value into dest through the pgtype codec registered
// for the column's OID. The mocked rows hold Go values rather than wire bytes,
// so the value is encoded and decoded again - the same round trip a real
// connection performs, which is what makes a registered custom type behave
// here as it does against a server.
//
// Columns without a DataTypeOID cannot be routed through pgtype at all and
// report errNoTypeMapping.
func (rs *rowSets) scanViaTypeMap(fd pgconn.FieldDescription, value, dest any) error {
	if fd.DataTypeOID == 0 {
		return errNoTypeMapping
	}
	m := rs.getTypeMap()
	encoded, err := m.encode(fd.DataTypeOID, fd.Format, value)
	if err != nil {
		return err
	}
	return m.scan(fd.DataTypeOID, fd.Format, encoded, dest)
}

// defaultTypeMap serves rows that were not produced by a query and therefore
// have no mock to borrow a type map from.
var defaultTypeMap = newLockedTypeMap()

// RawValues attempts to return the binary representation of the row values as
// if postgres had returned them. RawValues will consolidate the column OIDs and
// FormatCode.
//
// It is expected that if you are testing with RawValues, you know what you're
// doing in terms of how postgres will marshal certain data types into binary
// format, such as numerics, dates, and timestamps, etc.
//
// See https://github.com/jackc/pgx/tree/ac0b46f2f9538baa74aeac931d97884e5b9c924d/pgtype
func (rs *rowSets) RawValues() [][]byte {
	r := rs.sets[rs.RowSetNo]
	dest := make([][]byte, len(r.defs))
	fd := rs.FieldDescriptions()

	m := rs.getTypeMap()
	for i, col := range r.rows[r.recNo-1] {
		encoded, err := m.encode(fd[i].DataTypeOID, fd[i].Format, col)
		if err != nil {
			// fallback to a %v conversion.
			dest[i] = fmt.Appendf(nil, "%v", col)
			continue
		}

		dest[i] = encoded
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

// Rows is a mocked collection of rows to
// return for Query result
type Rows struct {
	commandTag pgconn.CommandTag
	defs       []pgconn.FieldDescription
	rows       [][]any
	recNo      int
	nextErr    map[int]error
	closeErr   error
	closed     bool
}

// NewRows allows Rows to be created from a slice of column names. Values are
// then added with Rows.AddRow, Rows.AddRows or Rows.FromCSVString. Use
// NewRowsWithColumnDefinition when the columns need metadata beyond a name.
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

// CloseError sets an error which will be returned by [Rows.Err] after
// [Rows.Close] has been called or [Rows.Next] returns false.
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

	row := make([]any, len(r.defs))
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

		row := make([]any, len(r.defs))
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

// clone returns a copy of r that can be iterated independently of the
// original. Column definitions and row values are immutable once the
// expectation is set up, so they are shared; the per-iteration cursor state
// is not.
func (r *Rows) clone() *Rows {
	c := *r
	c.nextErr = make(map[int]error, len(r.nextErr))
	maps.Copy(c.nextErr, r.nextErr)
	return &c
}
