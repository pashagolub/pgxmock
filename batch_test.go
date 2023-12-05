package pgxmock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBatchClosed(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	expectedBatch := mock.NewBatch().
		AddBatchElements(
			NewBatchElement("SELECT *", 1),
			NewBatchElement("SELECT *"),
		)

	batchResultsMock := NewBatchResults()

	batch := new(pgx.Batch)
	batch.Queue("SELECT * FROM TABLE", 1)
	batch.Queue("SELECT * FROM TABLE")

	mock.ExpectSendBatch(expectedBatch).
		WillReturnBatchResults(batchResultsMock)

	br := mock.SendBatch(context.Background(), batch)
	a.NotNil(br)
	a.NoError(br.Close())

	a.NoError(mock.ExpectationsWereMet())
}

func TestBatchWithRewrittenSQL(t *testing.T) {
	t.Parallel()
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	a := assert.New(t)
	a.NoError(err)
	defer mock.Close(context.Background())

	u := user{name: "John", email: pgtype.Text{String: "john@example.com", Valid: true}}

	expectedBatch := mock.NewBatch().
		AddBatchElements(
			//first batch query is correct
			NewBatchElement("INSERT", &u).
				WithRewrittenSQL("INSERT INTO users (username, email) VALUES ($1, $2) RETURNING id"),
			//second batch query is not correct
			NewBatchElement("INSERT INTO users(username, password) VALUES (@user, @password)", pgx.NamedArgs{"user": "John", "password": "strong"}).
				WithRewrittenSQL("INSERT INTO users(username, password) VALUES ($1)"),
		)
	batchResultsMock := NewBatchResults()

	mock.ExpectSendBatch(expectedBatch).
		WillReturnBatchResults(batchResultsMock).
		BatchResultsWillBeClosed()

	batch := new(pgx.Batch)
	batch.Queue("INSERT", &u)
	batch.Queue("INSERT INTO users(username) VALUES (@user)", pgx.NamedArgs{"user": "John", "password": "strong"})

	br := mock.SendBatch(context.Background(), batch)
	a.Nil(br)
	a.Error(mock.ExpectationsWereMet())
}

func TestBatchQuery(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	expectedBatch := mock.NewBatch().
		AddBatchElements(
			NewBatchElement("SELECT *", 1),
			NewBatchElement("SELECT *"),
		)

	rows := NewRows([]string{"id", "name", "email"}).
		AddRow("some-id-1", "some-name-1", "some-email-1").
		AddRow("some-id-2", "some-name-2", "some-email-2")

	batchResultsMock := NewBatchResults().WillReturnRows(rows).AddCommandTag(pgconn.NewCommandTag("SELECT 2"))

	batch := new(pgx.Batch)
	batch.Queue("SELECT * FROM TABLE", 1)
	batch.Queue("SELECT * FROM TABLE")

	mock.ExpectSendBatch(expectedBatch).
		WillReturnBatchResults(batchResultsMock)

	br := mock.SendBatch(context.Background(), batch)
	a.NotNil(br)
	r, err := br.Query()
	a.NoError(err)

	//assert rows are returned correctly
	var id, name, email string
	err = r.Scan(&id, &name, &email)
	a.NoError(err)
	a.Equal("some-id-1", id)
	a.Equal("some-name-1", name)
	a.Equal("some-email-1", email)

	a.True(r.Next())
	a.NoError(mock.ExpectationsWereMet())
}

func TestBatchErrors(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	expectedBatch := mock.NewBatch().
		AddBatchElements(
			NewBatchElement("SELECT *", 1),
			NewBatchElement("SELECT *"),
		)

	batchResultsMock := NewBatchResults().
		QueryError(fmt.Errorf("query returned error")).
		ExecError(fmt.Errorf("exec returned error")).
		CloseError(fmt.Errorf("close returned error"))

	batch := new(pgx.Batch)
	batch.Queue("SELECT * FROM TABLE", 1)
	batch.Queue("SELECT * FROM TABLE")

	mock.ExpectSendBatch(expectedBatch).
		WillReturnBatchResults(batchResultsMock)

	br := mock.SendBatch(context.Background(), batch)
	a.NotNil(br)

	_, err = br.Query()
	a.Error(err)

	_, err = br.Exec()
	a.Error(err)

	err = br.Close()
	a.Error(err)

	a.NoError(mock.ExpectationsWereMet())
}

func TestBatchQueryRow(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	expectedBatch := mock.NewBatch().
		AddBatchElements(
			NewBatchElement("SELECT *", 1),
			NewBatchElement("SELECT *"),
		)

	rows := NewRows([]string{"id", "name", "email"}).
		AddRow("some-id-1", "some-name-1", "some-email-1").
		AddRow("some-id-2", "some-name-2", "some-email-2")

	batchResultsMock := NewBatchResults().WillReturnRows(rows)

	batch := new(pgx.Batch)
	batch.Queue("SELECT * FROM TABLE", 1)
	batch.Queue("SELECT * FROM TABLE")

	mock.ExpectSendBatch(expectedBatch).
		WillReturnBatchResults(batchResultsMock)

	br := mock.SendBatch(context.Background(), batch)
	a.NotNil(br)

	r := br.QueryRow()

	//assert rows are returned correctly
	var id, name, email string
	err = r.Scan(&id, &name, &email)
	a.NoError(err)
	a.Equal("some-id-1", id)
	a.Equal("some-name-1", name)
	a.Equal("some-email-1", email)

	a.NoError(mock.ExpectationsWereMet())
}
