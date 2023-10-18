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

func TestTimes(t *testing.T) {
	t.Parallel()
	mock, _ := NewConn()
	a := assert.New(t)
	mock.ExpectPing().Times(2)
	err := mock.Ping(ctx)
	a.NoError(err)
	a.Error(mock.ExpectationsWereMet()) // must be two Ping() calls
	err = mock.Ping(ctx)
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}

func TestMaybe(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	mock, _ := NewConn()
	a := assert.New(t)

	mock.ExpectPing().WillDelayFor(time.Second).Maybe().Times(4)

	c, f := context.WithCancel(ctx)
	f()
	a.Error(mock.Ping(c), "should raise error for cancelled context")

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
	ex.WillDelayFor(time.Second).Maybe().Times(2)

	fmt.Print(ex)
	res, _ := mock.Exec(ctx, "INSERT something")
	fmt.Println(res)
	ex.WithArgs(42)
	fmt.Print(ex)
	res, _ = mock.Exec(ctx, "INSERT something", 42)
	fmt.Print(res)
	// Output:
	// ExpectedExec => expecting call to Exec():
	// 	- matches sql: '^INSERT (.+)'
	// 	- is without arguments
	// 	- returns result: INSERT 15
	// 	- delayed execution for: 1s
	// 	- execution is optional
	// 	- execution calls awaited: 2
	// INSERT 15
	// ExpectedExec => expecting call to Exec():
	// 	- matches sql: '^INSERT (.+)'
	// 	- is with arguments:
	// 		0 - 42
	// 	- returns result: INSERT 15
	// 	- delayed execution for: 1s
	// 	- execution is optional
	// 	- execution calls awaited: 2
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
	a := assert.New(t)
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
	mock.ExpectQuery(query).WillReturnError(errors.New("oops"))
	mock.ExpectExec(query).WillReturnResult(NewResult("SELECT", 1))
	mock.ExpectPrepare("foo", query)

	err := mock.Ping(ctx)
	a.Error(err)
	mock.QueryRow(ctx, query)
	_, err = mock.Exec(ctx, query)
	a.NoError(err)
	_, err = mock.Prepare(ctx, "foo", query)
	a.NoError(err)

	a.NoError(mock.ExpectationsWereMet())
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

func TestWithRewrittenSQL(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	a := assert.New(t)
	a.NoError(err)

	mock.ExpectQuery(`INSERT INTO users\(username\) VALUES \(\@user\)`).
		WithArgs(pgx.NamedArgs{"user": "John"}).
		WithRewrittenSQL(`INSERT INTO users\(username\) VALUES \(\$1\)`).
		WillReturnRows()

	_, err = mock.Query(context.Background(),
		"INSERT INTO users(username) VALUES (@user)",
		pgx.NamedArgs{"user": "John"},
	)
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}
