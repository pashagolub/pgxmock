package pgxmock

import (
	"context"
	"testing"
	"time"
)

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
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
	// defer db.Close()

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

func TestByteSliceArgument(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	// defer db.Close()

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
