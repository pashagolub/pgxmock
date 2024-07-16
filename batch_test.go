package pgxmock

import (
	"errors"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestBatch(t *testing.T) {
	t.Parallel()
	mock, _ := NewConn()
	a := assert.New(t)

	// define our expectations
	eb := mock.ExpectBatch()
	eb.ExpectQuery("select").WillReturnRows(NewRows([]string{"sum"}).AddRow(2))
	eb.ExpectExec("update").WithArgs(true, 1).WillReturnResult(NewResult("UPDATE", 1))

	// run the test
	batch := &pgx.Batch{}
	batch.Queue("select 1 + 1").QueryRow(func(row pgx.Row) error {
		var n int
		return row.Scan(&n)
	})
	batch.Queue("update users set active = $1 where id = $2", true, 1).Exec(func(ct pgconn.CommandTag) (err error) {
		if ct.RowsAffected() != 1 {
			err = errors.New("expected 1 row to be affected")
		}
		return
	})

	err := mock.SendBatch(ctx, batch).Close()
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}

func TestExplicitBatch(t *testing.T) {
	t.Parallel()
	mock, _ := NewConn()
	a := assert.New(t)

	// define our expectations
	eb := mock.ExpectBatch()
	eb.ExpectQuery("select").WillReturnRows(NewRows([]string{"sum"}).AddRow(2))
	eb.ExpectQuery("select").WillReturnRows(NewRows([]string{"answer"}).AddRow(42))
	eb.ExpectExec("update").WithArgs(true, 1).WillReturnResult(NewResult("UPDATE", 1))

	// run the test
	batch := &pgx.Batch{}
	batch.Queue("select 1 + 1")
	batch.Queue("select 42")
	batch.Queue("update users set active = $1 where id = $2", true, 1)

	var sum int
	br := mock.SendBatch(ctx, batch)
	err := br.QueryRow().Scan(&sum)
	a.NoError(err)
	a.Equal(2, sum)

	var answer int
	rows, err := br.Query()
	a.NoError(err)
	rows.Next()
	err = rows.Scan(&answer)
	a.NoError(err)
	a.Equal(42, answer)

	ct, err := br.Exec()
	a.NoError(err)
	a.True(ct.Update())
	a.EqualValues(1, ct.RowsAffected())

	// no more queries
	_, err = br.Exec()
	a.Error(err)
	_, err = br.Query()
	a.Error(err)
	err = br.QueryRow().Scan(&sum)
	a.Error(err)

	a.NoError(mock.ExpectationsWereMet())
}

func processBatch(db PgxPoolIface) error {
	batch := &pgx.Batch{}
	// Random order
	batch.Queue("SELECT id FROM normalized_queries WHERE query = $1", "some query")
	batch.Queue("INSERT INTO normalized_queries (query) VALUES ($1) RETURNING id", "some query")

	results := db.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < batch.Len(); i++ {
		var id int
		err := results.QueryRow().Scan(&id)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestUnorderedBatchExpectations(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	mock, err := NewPool()
	a.NoError(err)
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)

	expectedBatch := mock.ExpectBatch()
	expectedBatch.ExpectQuery("INSERT INTO").WithArgs("some query").
		WillReturnRows(NewRows([]string{"id"}).AddRow(10))
	expectedBatch.ExpectQuery("SELECT id").WithArgs("some query").
		WillReturnRows(NewRows([]string{"id"}).AddRow(20))

	err = processBatch(mock)
	a.NoError(err)
	a.NoError(mock.ExpectationsWereMet())
}
