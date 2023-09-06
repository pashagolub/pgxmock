package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestMaybe(t *testing.T) {
	mock, _ := NewConn()
	a := assert.New(t)
	mock.ExpectPing().Maybe()
	mock.ExpectBegin().Maybe()
	mock.ExpectQuery("SET TIME ZONE 'Europe/Rome'").Maybe() //only if we're in Italy
	cmdtag := pgconn.NewCommandTag("SELECT 1")
	mock.ExpectExec("select").WillReturnResult(cmdtag)
	mock.ExpectCommit().Maybe()

	res, err := mock.Exec(ctx, "select version()")
	a.Equal(cmdtag, res)
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}

func TestPanic(t *testing.T) {
	mock, _ := NewConn()
	a := assert.New(t)
	defer func() {
		a.NotNil(recover(), "The code did not panic")
		a.NoError(mock.ExpectationsWereMet())
	}()

	ex := mock.ExpectPing()
	ex.WillPanic("i'm tired")
	fmt.Println(ex)
	a.NoError(mock.Ping(ctx))
}

func TestCallModifier(t *testing.T) {
	mock, _ := NewConn()
	a := assert.New(t)

	mock.ExpectPing().WillDelayFor(time.Second).Maybe().Times(4)
	a.NoError(mock.ExpectationsWereMet()) //should produce no error since Ping() call is optional

	a.NoError(mock.Ping(ctx))
	a.NoError(mock.ExpectationsWereMet()) //should produce no error since Ping() was called actually
}

func TestCopyFromBug(t *testing.T) {
	mock, _ := NewConn()
	a := assert.New(t)

	mock.ExpectCopyFrom(pgx.Identifier{"foo"}, []string{"bar"}).WillReturnResult(1)

	var rows [][]any
	rows = append(rows, []any{"baz"})

	r, err := mock.CopyFrom(ctx, pgx.Identifier{"foo"}, []string{"bar"}, pgx.CopyFromRows(rows))
	a.EqualValues(len(rows), r)
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}

func ExampleExpectedExec() {
	mock, _ := NewConn()
	ex := mock.ExpectExec("^INSERT (.+)").WillReturnResult(NewResult("INSERT", 15))
	ex.WillDelayFor(time.Second)
	fmt.Print(ex)
	res, _ := mock.Exec(ctx, "INSERT something")
	fmt.Println(res)
	// Output: ExpectedExec => expecting call to Exec():
	// 	- matches sql: '^INSERT (.+)'
	// 	- is without arguments
	// 	- returns result: INSERT 15
	// 	- delayed execution for: 1s
	// INSERT 15
}

func TestUnexpectedPing(t *testing.T) {
	mock, _ := NewConn()
	err := mock.Ping(ctx)
	if err == nil {
		t.Error("Ping should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	err = mock.Ping(ctx)
	if err == nil {
		t.Error("Ping should return error for unexpected call")
	}
}

func TestUnexpectedPrepare(t *testing.T) {
	mock, _ := NewConn()
	_, err := mock.Prepare(ctx, "foo", "bar")
	if err == nil {
		t.Error("Prepare should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	_, err = mock.Prepare(ctx, "foo", "bar")
	if err == nil {
		t.Error("Prepare should return error for unexpected call")
	}
}

func TestUnexpectedCopyFrom(t *testing.T) {
	mock, _ := NewConn()
	_, err := mock.CopyFrom(ctx, pgx.Identifier{"schema", "table"}, []string{"foo", "bar"}, nil)
	if err == nil {
		t.Error("CopyFrom should return error for unexpected call")
	}
	mock.ExpectExec("foo")
	_, err = mock.CopyFrom(ctx, pgx.Identifier{"schema", "table"}, []string{"foo", "bar"}, nil)
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

	_ = mock.Ping(ctx)
	mock.QueryRow(ctx, query)
	_, _ = mock.Exec(ctx, query)
	_, _ = mock.Prepare(ctx, "foo", query)

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
	row := mock.QueryRow(ctx, query)
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
	_, err := mock.Exec(ctx, "INSERT something", "something")
	if err == nil {
		t.Error("arguments do not match error was expected")
	}
	if err := mock.ExpectationsWereMet(); err == nil {
		t.Error("expectation was not matched error was expected")
	}
}
