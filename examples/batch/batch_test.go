package main

import (
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// a successful test case
func TestExpectBatch(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBatch()

	// Setup the example
	var example = ExampleBatch{db: mock, batch: &pgx.Batch{}}

	// now we execute our method
	example.requestBatch()

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// a failing test case
func TestExpectBegin(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()

	// Setup the example
	var example = ExampleBatch{db: mock, batch: &pgx.Batch{}}

	// now we execute our method
	example.requestBatch()

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
