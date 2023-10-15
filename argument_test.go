package pgxmock

import (
	"context"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
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
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
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
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
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
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
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

func TestByteSliceNamedArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}

	username := []byte("user")
	mock.ExpectExec(`INSERT INTO users\(username\) VALUES \(\@user\)`).
		WithArgs(pgx.NamedArgs{"user": username}).
		WithRewrittenSQL("INSERT INTO users(username) VALUES ($1)").
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
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
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
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
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
