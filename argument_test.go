package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

type AnyTime struct{}

// Match satisfies pgxmock.Argument interface
func (a AnyTime) Match(v interface{}) bool {
	_, ok := v.(time.Time)
	return ok
}

func TestAnyTimeArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs("john", AnyTime{}).
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(), "INSERT INTO users(name, created_at) VALUES (?, ?)", "john", time.Now())
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAnyTimeNamedArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(pgx.NamedArgs{"name": "john", "time": AnyTime{}}).
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(),
		"INSERT INTO users(name, created_at) VALUES (@name, @time)",
		pgx.NamedArgs{"name": "john", "time": time.Now()},
	)
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestByteSliceArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	username := []byte("user")
	mock.ExpectExec("INSERT INTO users").WithArgs(username).WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(), "INSERT INTO users(username) VALUES (?)", username)
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

type failQryRW struct {
	pgx.QueryRewriter
}

func (fqrw failQryRW) RewriteQuery(_ context.Context, _ *pgx.Conn, sql string, _ []any) (newSQL string, newArgs []any, err error) {
	return "", nil, errors.New("cannot rewrite query " + sql)
}

func TestExpectQueryRewriterFail(t *testing.T) {

	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectQuery(`INSERT INTO users\(username\) VALUES \(\@user\)`).
		WithRewrittenSQL(`INSERT INTO users\(username\) VALUES \(\$1\)`).
		WithArgs(failQryRW{})
	_, err = mock.Query(context.Background(), "INSERT INTO users(username) VALUES (@user)", "baz")
	assert.Error(t, err)
}

func TestQueryRewriterFail(t *testing.T) {

	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	mock.ExpectExec(`INSERT INTO .+`).WithArgs("foo")
	_, err = mock.Exec(context.Background(), "INSERT INTO users(username) VALUES (@user)", failQryRW{})
	assert.Error(t, err)

}

func TestByteSliceNamedArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	username := []byte("user")
	mock.ExpectExec(`INSERT INTO users\(username\) VALUES \(\@user\)`).
		WithArgs(pgx.NamedArgs{"user": username}).
		WithRewrittenSQL(`INSERT INTO users\(username\) VALUES \(\$1\)`).
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(),
		"INSERT INTO users(username) VALUES (@user)",
		pgx.NamedArgs{"user": username},
	)
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAnyArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs("john", AnyArg()).
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(), "INSERT INTO users(name, created_at) VALUES (?, ?)", "john", time.Now())
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestAnyNamedArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs("john", AnyArg()).
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(), "INSERT INTO users(name, created_at) VALUES (@name, @created)",
		pgx.NamedArgs{"name": "john", "created": time.Now()},
	)
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

type panicArg struct{}

var panicArgErr = errors.New("this is a panic argument")

func (p panicArg) Match(_ any) bool {
	// This will always panic when called
	panic(panicArgErr)
}

var _ Argument = panicArg{}

func TestCloseAfterArgumentPanic(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	defer func() {
		checkFinishedWithin(t, 1*time.Second, func(ctx context.Context) {
			_ = mock.Close(ctx)
		})
	}()

	mock.ExpectExec("INSERT INTO users").
		WithArgs(panicArg{}).
		WillReturnResult(NewResult("INSERT", 1))

	assert.PanicsWithValue(t, panicArgErr, func() {
		_, _ = mock.Exec(context.Background(), "INSERT INTO users(name) VALUES (@name)",
			pgx.NamedArgs{"name": "john"},
		)
	})
}

func checkFinishedWithin(t *testing.T, timeout time.Duration, fun func(ctx context.Context)) {
	t.Helper()
	closeCtx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	finishedChan := make(chan bool)
	go func() {
		defer func() {
			finishedChan <- true
			close(finishedChan)
		}()
		defer func() {
			recover()
		}()
		fun(closeCtx)
	}()
	select {
	case <-finishedChan:
		return
	case <-closeCtx.Done():
		t.Error("timed out waiting for function to finish")
	}
}
