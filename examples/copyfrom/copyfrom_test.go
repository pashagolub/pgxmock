package main

import (
	"context"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func TestImportUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectCopyFrom(pgx.Identifier{"users"}, []string{"name", "age"}).
		WithRows(pgxmock.NewCopyRows("name", "age").
			AddRow("alice", 30).
			AddRow("bob", 40)).
		WillReturnResult(2)

	if err := importUsers(context.Background(), mock, []user{{"alice", 30}, {"bob", 40}}); err != nil {
		t.Errorf("error was not expected: %s", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestImportUsersReportsShortCopy(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectCopyFrom(pgx.Identifier{"users"}, []string{"name", "age"}).WillReturnResult(1)

	if err := importUsers(context.Background(), mock, []user{{"alice", 30}, {"bob", 40}}); err == nil {
		t.Error("an error was expected when fewer rows are copied")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
