package main

import (
	"testing"

	"github.com/pashagolub/pgxmock/v3"
)

// a successful test case
func TestShouldSelectRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// TODO
}

// a failing test case
func TestShouldRollbackOnFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	// TODO: mock.ExpectBatch()
	mock.ExpectRollback()

	// now we execute our method
	if err = requestBatch(mock); err == nil {
		t.Errorf("was expecting an error, but there was none")
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
