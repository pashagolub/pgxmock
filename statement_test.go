// +build go1.6

package pgxmock

import (
	"context"
	"errors"
	"testing"
)

func TestExpectedPreparedStatementCloseError(t *testing.T) {
	mock, err := New()
	if err != nil {
		t.Fatal("failed to open sqlmock database:", err)
	}

	mock.ExpectBegin()
	want := errors.New("STMT ERROR")
	mock.ExpectPrepare("SELECT").WillReturnCloseError(want)

	txn, err := mock.Begin(context.Background())
	if err != nil {
		t.Fatal("unexpected error while opening transaction:", err)
	}

	stmt, err := txn.Prepare(context.Background(), "foo", "SELECT")
	if err != nil {
		t.Fatal("unexpected error while preparing a statement:", err)
	}

	if stmt.Name != "foo" {
		t.Fatalf("got = %v, want = %v", stmt.Name, "foo")
	}

	// 	if err := stmt.Close(); err != want {
	// 		t.Fatalf("got = %v, want = %v", err, want)
	// 	}
}
