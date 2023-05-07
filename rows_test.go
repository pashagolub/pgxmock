package pgxmock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPointerToInterfaceArgument(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		panic(err)
	}

	mock.ExpectQuery(`SELECT 123`).
		WillReturnRows(
			mock.NewRows([]string{"id"}).
				AddRow(int64(123))) // Value which should be scanned in *interface{}

	var value interface{}
	err = mock.QueryRow(context.Background(), `SELECT 123`).Scan(&value)
	if err != nil || value.(int64) != 123 {
		t.Error(err)
	}

}

func TestExplicitTypeCasting(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		panic(err)
	}

	mock.ExpectQuery("SELECT .+ FROM test WHERE .+").
		WithArgs(uint64(1)).
		WillReturnRows(NewRows(
			[]string{"id"}).
			AddRow(uint64(1)),
		)

	rows := mock.QueryRow(context.Background(), "SELECT id FROM test WHERE id = $1", uint64(1))

	var id uint64
	err = rows.Scan(&id)
	if err != nil {
		t.Error(err)
	}
}

func TestAddRows(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open sqlmock database:", err)
	}
	defer mock.Close(context.Background())

	values := [][]any{
		{
			1, "John",
		},
		{
			2, "Jane",
		},
		{
			3, "Peter",
		},
		{
			4, "Emily",
		},
	}

	rows := NewRows([]string{"id", "name"}).AddRows(values...)
	mock.ExpectQuery("SELECT").WillReturnRows(rows).RowsWillBeClosed()

	rs, _ := mock.Query(context.Background(), "SELECT")
	defer rs.Close()

	for rs.Next() {
		var id int
		var name string
		_ = rs.Scan(&id, &name)
		fmt.Println("scanned id:", id, "and name:", name)
	}

	if rs.Err() != nil {
		fmt.Println("got rows error:", rs.Err())
	}
	// Output: scanned id: 1 and title: John
	// scanned id: 2 and title: Jane
	// scanned id: 3 and title: Peter
	// scanned id: 4 and title: Emily
}

func ExampleRows_AddRows() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open sqlmock database:", err)
	}
	defer mock.Close(context.Background())

	values := [][]any{
		{
			1, "one",
		},
		{
			2, "two",
		},
	}

	rows := NewRows([]string{"id", "title"}).AddRows(values...)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, _ := mock.Query(context.Background(), "SELECT")
	defer rs.Close()

	for rs.Next() {
		var id int
		var title string
		_ = rs.Scan(&id, &title)
		fmt.Println("scanned id:", id, "and title:", title)
	}

	if rs.Err() != nil {
		fmt.Println("got rows error:", rs.Err())
	}
	// Output: scanned id: 1 and title: one
	// scanned id: 2 and title: two
}

func ExampleRows() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id", "title"}).
		AddRow(1, "one").
		AddRow(2, "two").
		AddCommandTag(pgconn.NewCommandTag("SELECT 2"))

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, _ := mock.Query(context.Background(), "SELECT")
	defer rs.Close()

	fmt.Println("command tag:", rs.CommandTag())
	if len(rs.FieldDescriptions()) != 2 {
		fmt.Println("got wrong number of fields")
	}

	for rs.Next() {
		var id int
		var title string
		_ = rs.Scan(&id, &title)
		fmt.Println("scanned id:", id, "and title:", title)
	}

	if rs.Err() != nil {
		fmt.Println("got rows error:", rs.Err())
	}

	// Output: command tag: SELECT 2
	// scanned id: 1 and title: one
	// scanned id: 2 and title: two
}

func ExampleRows_rowError() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	// defer mock.Close(context.Background())

	rows := NewRows([]string{"id", "title"}).
		AddRow(0, "one").
		AddRow(1, "two").
		RowError(1, fmt.Errorf("row error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, _ := mock.Query(context.Background(), "SELECT")
	defer rs.Close()

	for rs.Next() {
		var id int
		var title string
		_ = rs.Scan(&id, &title)
		fmt.Println("scanned id:", id, "and title:", title)
		if rs.Err() != nil {
			fmt.Println("got rows error:", rs.Err())
		}
	}

	// Output: scanned id: 0 and title: one
	// scanned id: 1 and title: two
	// got rows error: row error
}

func ExampleRows_expectToBeClosed() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	defer mock.Close(context.Background())

	row := NewRows([]string{"id", "title"}).AddRow(1, "john")
	rows := NewRowsWithColumnDefinition(
		pgconn.FieldDescription{Name: "id"},
		pgconn.FieldDescription{Name: "title"}).
		AddRow(1, "john").AddRow(2, "anna")
	mock.ExpectQuery("SELECT").WillReturnRows(row, rows).RowsWillBeClosed()

	_, _ = mock.Query(context.Background(), "SELECT")
	_, _ = mock.Query(context.Background(), "SELECT")

	if err := mock.ExpectationsWereMet(); err != nil {
		fmt.Println("got error:", err)
	}

	// Output: got error: expected query rows to be closed, but it was not: ExpectedQuery => expecting Query, QueryContext or QueryRow which:
	//   - matches sql: 'SELECT'
	//   - is without arguments
	//   - should return rows:
	//     result set: 0
	//       row 0 - [1 john]
	//     result set: 1
	//       row 0 - [1 john]
	//       row 1 - [2 anna]
}

func ExampleRows_customDriverValue() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id", "null_int"}).
		AddRow(5, pgtype.Int8{Int64: 5, Valid: true}).
		AddRow(2, pgtype.Int8{Valid: false})

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, _ := mock.Query(context.Background(), "SELECT")
	defer rs.Close()

	for rs.Next() {
		var id int
		var num pgtype.Int8
		_ = rs.Scan(&id, &num)
		fmt.Println("scanned id:", id, "and null int64:", num)
	}

	if rs.Err() != nil {
		fmt.Println("got rows error:", rs.Err())
	}
	// Output: scanned id: 5 and null int64: {5 true}
	// scanned id: 2 and null int64: {0 false}
}

func TestAllowsToSetRowsErrors(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id", "title"}).
		AddRow(0, "one").
		AddRow(1, "two").
		RowError(1, fmt.Errorf("error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer rs.Close()

	if !rs.Next() {
		t.Fatal("expected the first row to be available")
	}
	if rs.Err() != nil {
		t.Fatalf("unexpected error: %s", rs.Err())
	}

	if !rs.Next() {
		t.Fatal("expected the second row to be available, even there should be an error")
	}
	if rs.Err() == nil {
		t.Fatal("expected an error, but got none")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRowsCloseError(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id"}).CloseError(fmt.Errorf("close error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rs.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRowsClosed(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery("SELECT").WillReturnRows(rows).RowsWillBeClosed()

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rs.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQuerySingleRow(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	var id int
	if err := mock.QueryRow(context.Background(), "SELECT").Scan(&id); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}))
	if err := mock.QueryRow(context.Background(), "SELECT").Scan(&id); err != pgx.ErrNoRows {
		t.Fatal("expected sql no rows error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func ExampleRows_values() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"raw"}).
		AddRow(`one string value with some text!`).
		AddRow(`two string value with even more text than the first one`).
		AddRow([]byte{})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		fmt.Print(err)
	}
	defer rs.Close()

	for rs.Next() {
		v, e := rs.Values()
		fmt.Println(v[0], e)
	}
	// Output: one string value with some text! <nil>
	// two string value with even more text than the first one <nil>
	// [] <nil>
}

func ExampleRows_rawValues() {
	mock, err := NewConn()
	if err != nil {
		fmt.Println("failed to open pgxmock database:", err)
	}
	defer mock.Close(context.Background())

	rows := NewRows([]string{"raw"}).
		AddRow([]byte(`one binary value with some text!`)).
		AddRow([]byte(`two binary value with even more text than the first one`)).
		AddRow([]byte{})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		fmt.Print(err)
	}
	defer rs.Close()

	for rs.Next() {
		var rawValue []byte
		if err := json.Unmarshal(rs.RawValues()[0], &rawValue); err != nil {
			fmt.Print(err)
		}
		fmt.Println(string(rawValue))
	}
	// Output: one binary value with some text!
	// two binary value with even more text than the first one
	//
}

// func TestQueryRowBytesNotInvalidatedByNext_bytesIntoBytes(t *testing.T) {
// 	t.Parallel()
// 	rows := NewRows([]string{"raw"}).
// 		AddRow([]byte(`one binary value with some text!`)).
// 		AddRow([]byte(`two binary value with even more text than the first one`))
// 	scan := func(rs *sql.Rows) ([]byte, error) {
// 		var b []byte
// 		return b, rs.Scan(&b)
// 	}
// 	want := [][]byte{[]byte(`one binary value with some text!`), []byte(`two binary value with even more text than the first one`)}
// 	queryRowBytesNotInvalidatedByNext(t, rows, scan, want)
// }

// func TestQueryRowBytesNotInvalidatedByNext_stringIntoBytes(t *testing.T) {
// 	t.Parallel()
// 	rows := NewRows([]string{"raw"}).
// 		AddRow(`one binary value with some text!`).
// 		AddRow(`two binary value with even more text than the first one`)
// 	scan := func(rs *sql.Rows) ([]byte, error) {
// 		var b []byte
// 		return b, rs.Scan(&b)
// 	}
// 	want := [][]byte{[]byte(`one binary value with some text!`), []byte(`two binary value with even more text than the first one`)}
// 	queryRowBytesNotInvalidatedByNext(t, rows, scan, want)
// }

// func TestQueryRowBytesInvalidatedByClose_bytesIntoRawBytes(t *testing.T) {
// 	t.Parallel()
// 	replace := []byte(invalid)
// 	rows := NewRows([]string{"raw"}).AddRow([]byte(`one binary value with some text!`))
// 	scan := func(rs *sql.Rows) ([]byte, error) {
// 		var raw sql.RawBytes
// 		return raw, rs.Scan(&raw)
// 	}
// 	want := struct {
// 		Initial  []byte
// 		Replaced []byte
// 	}{
// 		Initial:  []byte(`one binary value with some text!`),
// 		Replaced: replace[:len(replace)-7],
// 	}
// 	queryRowBytesInvalidatedByClose(t, rows, scan, want)
// }

// func TestQueryRowBytesNotInvalidatedByClose_bytesIntoBytes(t *testing.T) {
// 	t.Parallel()
// 	rows := NewRows([]string{"raw"}).AddRow([]byte(`one binary value with some text!`))
// 	scan := func(rs *sql.Rows) ([]byte, error) {
// 		var b []byte
// 		return b, rs.Scan(&b)
// 	}
// 	queryRowBytesNotInvalidatedByClose(t, rows, scan, []byte(`one binary value with some text!`))
// }

// func TestQueryRowBytesNotInvalidatedByClose_stringIntoBytes(t *testing.T) {
// 	t.Parallel()
// 	rows := NewRows([]string{"raw"}).AddRow(`one binary value with some text!`)
// 	scan := func(rs *sql.Rows) ([]byte, error) {
// 		var b []byte
// 		return b, rs.Scan(&b)
// 	}
// 	queryRowBytesNotInvalidatedByClose(t, rows, scan, []byte(`one binary value with some text!`))
// }

func TestRowsScanError(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	r := NewRows([]string{"col1", "col2"}).AddRow("one", "two").AddRow("one", nil)
	mock.ExpectQuery("SELECT").WillReturnRows(r)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer rs.Close()

	var one, two string
	if !rs.Next() || rs.Err() != nil || rs.Scan(&one, &two) != nil {
		t.Fatal("unexpected error on first row scan")
	}

	if !rs.Next() || rs.Err() != nil {
		t.Fatal("unexpected error on second row read")
	}

	err = rs.Scan(&one, two)
	if err == nil {
		t.Fatal("expected an error for scan, but got none")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type testScanner struct {
	Value int64
}

func (s *testScanner) Scan(src interface{}) error {
	switch src := src.(type) {
	case int64:
		s.Value = src
		return nil
	default:
		return errors.New("a dummy scan error")
	}
}

func TestRowsScanWithScannerIface(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	r := NewRows([]string{"col1"}).AddRow(int64(23))
	mock.ExpectQuery("SELECT").WillReturnRows(r)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var result testScanner
	if !rs.Next() || rs.Err() != nil {
		t.Fatal("unexpected error on first row read")
	}
	if rs.Scan(&result) != nil {
		t.Fatal("unexpected error for scan")
	}

	if result.Value != int64(23) {
		t.Fatalf("expected Value to be 23 but got: %d", result.Value)
	}

}

func TestRowsScanErrorOnScannerIface(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	r := NewRows([]string{"col1"}).AddRow("one").AddRow("two")
	mock.ExpectQuery("SELECT").WillReturnRows(r)

	rs, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var one int64       // No scanner interface
	var two testScanner // scanner error
	if !rs.Next() || rs.Err() != nil {
		t.Fatal("unexpected error on first row read")
	}
	if rs.Scan(&one) == nil {
		t.Fatal("expected an error for first scan (no scanner interface), but got none")
	}

	if !rs.Next() || rs.Err() != nil {
		t.Fatal("unexpected error on second row read")
	}

	err = rs.Scan(&two)
	if err == nil {
		t.Fatal("expected an error for second scan (scanner error), but got none")
	}
}

func TestCSVRowParser(t *testing.T) {
	t.Parallel()
	rs := NewRows([]string{"col1", "col2"}).FromCSVString("a,NULL")
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectQuery("SELECT").WillReturnRows(rs)

	rw, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer rw.Close()
	var col1 string
	var col2 []byte

	rw.Next()
	if err = rw.Scan(&col1, &col2); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if col1 != "a" {
		t.Fatalf("expected col1 to be 'a', but got [%T]:%+v", col1, col1)
	}
	if col2 != nil {
		t.Fatalf("expected col2 to be nil, but got [%T]:%+v", col2, col2)
	}
}

func TestWrongNumberOfValues(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	defer mock.Close(context.Background())
	defer func() {
		_ = recover()
	}()
	mock.ExpectQuery("SELECT ID FROM TABLE").WithArgs(101).WillReturnRows(NewRows([]string{"ID"}).AddRow(101, "Hello"))
	_, _ = mock.Query(context.Background(), "SELECT ID FROM TABLE", 101)
	// shouldn't reach here
	t.Error("expected panic from query")
}

func TestEmptyRowSets(t *testing.T) {
	rs1 := NewRows([]string{"a"}).AddRow("a")
	rs2 := NewRows([]string{"b"})
	rs3 := NewRows([]string{"c"})

	set1 := &rowSets{sets: []*Rows{rs1, rs2}}
	set2 := &rowSets{sets: []*Rows{rs3, rs2}}
	set3 := &rowSets{sets: []*Rows{rs2}}

	if set1.empty() {
		t.Fatalf("expected rowset 1, not to be empty, but it was")
	}
	if !set2.empty() {
		t.Fatalf("expected rowset 2, to be empty, but it was not")
	}
	if !set3.empty() {
		t.Fatalf("expected rowset 3, to be empty, but it was not")
	}
}

// func queryRowBytesInvalidatedByNext(t *testing.T, rows *Rows, scan func(*sql.Rows) ([]byte, error), want []struct {
// 	Initial  []byte
// 	Replaced []byte
// }) {
// 	mock, err := New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer mock.Close(context.Background())
// 	mock.ExpectQuery("SELECT").WillReturnRows(rows)

// 	rs, err := mock.Query(context.Background(), "SELECT")
// 	if err != nil {
// 		t.Fatalf("failed to query rows: %s", err)
// 	}

// 	if !rs.Next() || rs.Err() != nil {
// 		t.Fatal("unexpected error on first row retrieval")
// 	}
// 	var count int
// 	for i := 0; ; i++ {
// 		count++
// 		b, err := scan(rs)
// 		if err != nil {
// 			t.Fatalf("unexpected error scanning row: %s", err)
// 		}
// 		if exp := want[i].Initial; !bytes.Equal(b, exp) {
// 			t.Fatalf("expected raw value to be '%s' (len:%d), but got [%T]:%s (len:%d)", exp, len(exp), b, b, len(b))
// 		}
// 		next := rs.Next()
// 		if exp := want[i].Replaced; !bytes.Equal(b, exp) {
// 			t.Fatalf("expected raw value to be replaced with '%s' (len:%d) after calling Next(), but got [%T]:%s (len:%d)", exp, len(exp), b, b, len(b))
// 		}
// 		if !next {
// 			break
// 		}
// 	}
// 	if err := rs.Err(); err != nil {
// 		t.Fatalf("row iteration failed: %s", err)
// 	}
// 	if exp := len(want); count != exp {
// 		t.Fatalf("incorrect number of rows exp: %d, but got %d", exp, count)
// 	}

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func queryRowBytesNotInvalidatedByNext(t *testing.T, rows *Rows, scan func(*sql.Rows) ([]byte, error), want [][]byte) {
// 	mock, err := New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer mock.Close(context.Background())
// 	mock.ExpectQuery("SELECT").WillReturnRows(rows)

// 	rs, err := mock.Query(context.Background(), "SELECT")
// 	if err != nil {
// 		t.Fatalf("failed to query rows: %s", err)
// 	}

// 	if !rs.Next() || rs.Err() != nil {
// 		t.Fatal("unexpected error on first row retrieval")
// 	}
// 	var count int
// 	for i := 0; ; i++ {
// 		count++
// 		b, err := scan(rs)
// 		if err != nil {
// 			t.Fatalf("unexpected error scanning row: %s", err)
// 		}
// 		if exp := want[i]; !bytes.Equal(b, exp) {
// 			t.Fatalf("expected raw value to be '%s' (len:%d), but got [%T]:%s (len:%d)", exp, len(exp), b, b, len(b))
// 		}
// 		next := rs.Next()
// 		if exp := want[i]; !bytes.Equal(b, exp) {
// 			t.Fatalf("expected raw value to be replaced with '%s' (len:%d) after calling Next(), but got [%T]:%s (len:%d)", exp, len(exp), b, b, len(b))
// 		}
// 		if !next {
// 			break
// 		}
// 	}
// 	if err := rs.Err(); err != nil {
// 		t.Fatalf("row iteration failed: %s", err)
// 	}
// 	if exp := len(want); count != exp {
// 		t.Fatalf("incorrect number of rows exp: %d, but got %d", exp, count)
// 	}

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func queryRowBytesInvalidatedByClose(t *testing.T, rows *Rows, scan func(*sql.Rows) ([]byte, error), want struct {
// 	Initial  []byte
// 	Replaced []byte
// }) {
// 	mock, err := New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer mock.Close(context.Background())
// 	mock.ExpectQuery("SELECT").WillReturnRows(rows)

// 	rs, err := mock.Query(context.Background(), "SELECT")
// 	if err != nil {
// 		t.Fatalf("failed to query rows: %s", err)
// 	}

// 	if !rs.Next() || rs.Err() != nil {
// 		t.Fatal("unexpected error on first row retrieval")
// 	}
// 	b, err := scan(rs)
// 	if err != nil {
// 		t.Fatalf("unexpected error scanning row: %s", err)
// 	}
// 	if !bytes.Equal(b, want.Initial) {
// 		t.Fatalf("expected raw value to be '%s' (len:%d), but got [%T]:%s (len:%d)", want.Initial, len(want.Initial), b, b, len(b))
// 	}
// 	rs.Close()

// 	if !bytes.Equal(b, want.Replaced) {
// 		t.Fatalf("expected raw value to be replaced with '%s' (len:%d) after calling Next(), but got [%T]:%s (len:%d)", want.Replaced, len(want.Replaced), b, b, len(b))
// 	}
// 	if err := rs.Err(); err != nil {
// 		t.Fatalf("row iteration failed: %s", err)
// 	}

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Fatal(err)
// 	}
// }

// func queryRowBytesNotInvalidatedByClose(t *testing.T, rows *Rows, scan func(*sql.Rows) ([]byte, error), want []byte) {
// 	mock, err := New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer mock.Close(context.Background())
// 	mock.ExpectQuery("SELECT").WillReturnRows(rows)

// 	rs, err := mock.Query(context.Background(), "SELECT")
// 	if err != nil {
// 		t.Fatalf("failed to query rows: %s", err)
// 	}

// 	if !rs.Next() || rs.Err() != nil {
// 		t.Fatal("unexpected error on first row retrieval")
// 	}
// 	b, err := scan(rs)
// 	if err != nil {
// 		t.Fatalf("unexpected error scanning row: %s", err)
// 	}
// 	if !bytes.Equal(b, want) {
// 		t.Fatalf("expected raw value to be '%s' (len:%d), but got [%T]:%s (len:%d)", want, len(want), b, b, len(b))
// 	}
// 	if err := rs.Close(); err != nil {
// 		t.Fatalf("unexpected error closing rows: %s", err)
// 	}
// 	if !bytes.Equal(b, want) {
// 		t.Fatalf("expected raw value to be replaced with '%s' (len:%d) after calling Next(), but got [%T]:%s (len:%d)", want, len(want), b, b, len(b))
// 	}
// 	if err := rs.Err(); err != nil {
// 		t.Fatalf("row iteration failed: %s", err)
// 	}

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Fatal(err)
// 	}
// }

func TestMockQueryWithCollect(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())
	type rowStructType struct {
		ID    int
		Title string
	}
	rs := NewRows([]string{"id", "title"}).AddRow(5, "hello world")

	mock.ExpectQuery("SELECT (.+) FROM articles WHERE id = ?").
		WithArgs(5).
		WillReturnRows(rs)

	rows, err := mock.Query(context.Background(), "SELECT (.+) FROM articles WHERE id = ?", 5)
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}

	defer rows.Close()

	//if !rows.Next() {
	//	t.Error("it must have had one row as result, but got empty result set instead")
	//}

	rawMap, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByPos[rowStructType])
	if err != nil {
		t.Errorf("error '%s' was not expected while trying to collect rows", err)
	}

	var id = rawMap[0].ID
	var title = rawMap[0].Title

	if err != nil {
		t.Errorf("error '%s' was not expected while trying to scan row", err)
	}

	if id != 5 {
		t.Errorf("expected mocked id to be 5, but got %d instead", id)
	}

	if title != "hello world" {
		t.Errorf("expected mocked title to be 'hello world', but got '%s' instead", title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
