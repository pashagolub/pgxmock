package main

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v5"
)

func TestTransfer(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	eb := mock.ExpectBatch()
	eb.ExpectExec("UPDATE accounts SET balance = balance -").
		WithArgs(100, 1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	eb.ExpectExec("UPDATE accounts SET balance = balance \\+").
		WithArgs(100, 2).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	eb.ExpectQuery("SELECT balance FROM accounts").
		WithArgs(1).
		WillReturnRows(mock.NewRows([]string{"balance"}).AddRow(400))

	balance, err := transfer(context.Background(), mock, 1, 2, 100)
	if err != nil {
		t.Fatalf("error was not expected: %s", err)
	}
	if balance != 400 {
		t.Errorf("expected balance 400, got %d", balance)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestTransferFromUnknownAccount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	eb := mock.ExpectBatch()
	eb.ExpectExec("UPDATE accounts SET balance = balance -").
		WithArgs(100, 42).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	// SendBatch checks every queued query, but the batch stops at the first
	// failure, so the rest is never read
	eb.ExpectExec("UPDATE accounts SET balance = balance \\+").WithArgs(100, 2).Maybe()
	eb.ExpectQuery("SELECT balance FROM accounts").WithArgs(42).Maybe()

	if _, err := transfer(context.Background(), mock, 42, 2, 100); err == nil {
		t.Error("an error was expected for an unknown account")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
