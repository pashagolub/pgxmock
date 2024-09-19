package main

import (
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestNewExample(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Expect a call to Exec
	mock.ExpectExec(`^CREATE TABLE IF NOT EXISTS (.+)`).
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))

	_, err = NewExample(mock)
	if err != nil {
		t.Errorf("creating new example error: %s", err)
	}

	// We make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSendCustomBatch(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Expect a call to Exec and pgx.Batch
	mock.ExpectExec(`^CREATE TABLE IF NOT EXISTS (.+)`).
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectBatch()

	example, err := NewExample(mock)
	if err != nil {
		t.Errorf("creating new example error: %s", err)
	}

	err = example.SendCustomBatch([]string{
		"SELECT title FROM metadata",
		"SELECT authors FROM metadata",
		"SELECT subject, description FROM metadata",
	})
	if err != nil {
		t.Errorf("SendCustomBatch error: %s", err)
	}

	err = example.TestCustomResults()
	if err != nil {
		t.Errorf("TestCustomResults error: %s", err)
	}

	// We make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
func TestBulkInsert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Expect a call to Exec and pgx.Batch
	mock.ExpectExec(`^CREATE TABLE IF NOT EXISTS (.+)`).
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectBatch()

	example, err := NewExample(mock)
	if err != nil {
		t.Errorf("creating new example error: %s", err)
	}

	// Insert multiple rows into the database
	err = example.BulkInsertMetadata([]metadata{
		{`title`, `author`, `subject`, `description`},
	})
	if err != nil {
		t.Errorf("bulk insert error: %s", err)
	}

	// We make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
