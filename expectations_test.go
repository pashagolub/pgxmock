package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPanic(t *testing.T) {
	mock, _ := NewConn()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("expectation were not met: %s", err)
		}
	}()

	ex := mock.ExpectPing()
	ex.WillPanic("i'm tired")
	fmt.Println(ex)
	if err := mock.Ping(context.Background()); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestCallModifier(t *testing.T) {
	mock, _ := NewConn()
	f := func() {
		err := mock.ExpectationsWereMet()
		if err != nil {
			t.Errorf("expectation were not met: %s", err)
		}
	}

	mock.ExpectPing().WillDelayFor(time.Second).Maybe().Times(4)
	f() //should produce no error since Ping() call is optional

	if err := mock.Ping(context.Background()); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	f() //should produce no error since Ping() was called actually
}

func TestCopyFromBug(t *testing.T) {
	mock, _ := NewConn()
	defer func() {
		err := mock.ExpectationsWereMet()
		if err != nil {
			t.Errorf("expectation were not met: %s", err)
		}
	}()

	mock.ExpectCopyFrom(pgx.Identifier{"foo"}, []string{"bar"}).WillReturnResult(1)

	var rows [][]any
	rows = append(rows, []any{"baz"})

	_, err := mock.CopyFrom(context.Background(), pgx.Identifier{"foo"}, []string{"bar"}, pgx.CopyFromRows(rows))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func ExampleExpectedExec() {
	mock, _ := NewConn()
	ex := mock.ExpectExec("^INSERT (.+)").WillReturnResult(NewResult("INSERT", 15))
	ex.WillDelayFor(time.Second)
	fmt.Print(ex)
	res, _ := mock.Exec(context.Background(), "INSERT something")
	fmt.Println(res)
	// Output: ExpectedExec => expecting call to Exec():
	// 	- matches sql: '^INSERT (.+)'
	// 	- is without arguments
	// 	- returns result: INSERT 15
	// 	- delayed execution for: 1s
	// INSERT 15
}

func TestUnmonitoredPing(t *testing.T) {
	mock, _ := NewConn()
	p := mock.ExpectPing()
	if p == nil {
		t.Error("ExpectPing should return *ExpectedPing")
	}
}

func TestUnexpectedPing(t *testing.T) {
	mock, _ := NewConn()
	err := mock.Ping(context.Background())
	if err == nil {
		t.Error("Ping should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	err = mock.Ping(context.Background())
	if err == nil {
		t.Error("Ping should return error for unexpected call")
	}
}

func TestUnexpectedPrepare(t *testing.T) {
	mock, _ := NewConn()
	_, err := mock.Prepare(context.Background(), "foo", "bar")
	if err == nil {
		t.Error("Prepare should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	_, err = mock.Prepare(context.Background(), "foo", "bar")
	if err == nil {
		t.Error("Prepare should return error for unexpected call")
	}
}

func TestUnexpectedCopyFrom(t *testing.T) {
	mock, _ := NewConn()
	_, err := mock.CopyFrom(context.Background(), pgx.Identifier{"schema", "table"}, []string{"foo", "bar"}, nil)
	if err == nil {
		t.Error("CopyFrom should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	_, err = mock.CopyFrom(context.Background(), pgx.Identifier{"schema", "table"}, []string{"foo", "bar"}, nil)
	if err == nil {
		t.Error("CopyFrom should return error for unexpected call")
	}
}

func TestBuildQuery(t *testing.T) {
	mock, _ := NewConn()
	query := `
		SELECT
			name,
			email,
			address,
			anotherfield
		FROM user
		where
			name    = 'John'
			and
			address = 'Jakarta'

	`

	mock.ExpectPing().WillDelayFor(1 * time.Second).WillReturnError(errors.New("no ping please"))
	mock.ExpectQuery(query)
	mock.ExpectExec(query)
	mock.ExpectPrepare("foo", query)

	_ = mock.Ping(context.Background())
	mock.QueryRow(context.Background(), query)
	_, _ = mock.Exec(context.Background(), query)
	_, _ = mock.Prepare(context.Background(), "foo", query)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestQueryRowScan(t *testing.T) {
	mock, _ := NewConn() //TODO New(ValueConverterOption(CustomConverter{}))
	query := `
		SELECT
			name,
			email,
			address,
			anotherfield
		FROM user
		where
			name    = 'John'
			and
			address = 'Jakarta'

	`
	expectedStringValue := "ValueOne"
	expectedIntValue := 2
	expectedArrayValue := []string{"Three", "Four"}
	mock.ExpectQuery(query).WillReturnRows(mock.NewRows([]string{"One", "Two", "Three"}).AddRow(expectedStringValue, expectedIntValue, []string{"Three", "Four"}))
	row := mock.QueryRow(context.Background(), query)
	var stringValue string
	var intValue int
	var arrayValue []string
	if e := row.Scan(&stringValue, &intValue, &arrayValue); e != nil {
		t.Error(e)
	}
	if stringValue != expectedStringValue {
		t.Errorf("Expectation %s does not met: %s", expectedStringValue, stringValue)
	}
	if intValue != expectedIntValue {
		t.Errorf("Expectation %d does not met: %d", expectedIntValue, intValue)
	}
	if !reflect.DeepEqual(expectedArrayValue, arrayValue) {
		t.Errorf("Expectation %v does not met: %v", expectedArrayValue, arrayValue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestMissingWithArgs(t *testing.T) {
	mock, _ := NewConn()
	// No arguments expected
	mock.ExpectExec("INSERT something")
	// Receiving argument
	_, err := mock.Exec(context.Background(), "INSERT something", "something")
	if err == nil {
		t.Error("arguments do not match error was expected")
	}
	if err := mock.ExpectationsWereMet(); err == nil {
		t.Error("expectation was not matched error was expected")
	}
}
