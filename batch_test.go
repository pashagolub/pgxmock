package pgxmock

import (
	"errors"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchCloseAdvancesPastUnreadQueries(t *testing.T) {
	for name, qq := range map[string]*pgx.QueuedQuery{
		"nil query":         nil,
		"callback not read": {SQL: "UPDATE", Fn: func(pgx.BatchResults) error { return nil }},
	} {
		t.Run(name, func(t *testing.T) {
			mock, _ := NewConn()
			mock.MatchExpectationsInOrder(false) // in order SendBatch would check the queued query up front
			mock.ExpectBatch().ExpectExec("UPDATE").Maybe()

			done := make(chan error)
			go func() { done <- mock.SendBatch(ctx, &pgx.Batch{QueuedQueries: []*pgx.QueuedQuery{qq}}).Close() }()
			select {
			case err := <-done:
				assert.NoError(t, err)
			case <-time.After(time.Second):
				t.Fatal("Close must not loop forever on a query it cannot advance past")
			}
		})
	}
}

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

// A failing queued query stops Close, as pgx does.
func TestBatchCloseStopsAtFirstError(t *testing.T) {
	errBoom := errors.New("boom")
	mock, _ := NewConn()
	eb := mock.ExpectBatch()
	eb.ExpectQuery("SELECT").WillReturnError(errBoom)
	eb.ExpectExec("UPDATE").Maybe().WillReturnResult(NewResult("UPDATE", 1))

	b := &pgx.Batch{}
	b.Queue("SELECT")
	b.Queue("UPDATE")
	assert.ErrorIs(t, mock.SendBatch(ctx, b).Close(), errBoom)
}

func TestBatchCloseReadsUnreadQueries(t *testing.T) {
	mock, _ := NewConn()
	eb := mock.ExpectBatch()
	eb.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}).AddRow(1)).RowsWillBeClosed()
	eb.ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))

	b := &pgx.Batch{}
	b.Queue("SELECT")
	b.Queue("UPDATE")
	assert.NoError(t, mock.SendBatch(ctx, b).Close())
	assert.NoError(t, mock.ExpectationsWereMet())
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

// SendBatch checks every queued query up front and fails the batch on a mismatch.
func TestSendBatchChecksQueuedQueries(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange func(*ExpectedBatch)
		queue   func(*pgx.Batch)
		want    string
	}{
		"sql": {
			arrange: func(eb *ExpectedBatch) { eb.ExpectExec("UPDATE") },
			queue:   func(b *pgx.Batch) { b.Queue("DELETE") },
			want:    "could not match actual sql",
		},
		"args": {
			arrange: func(eb *ExpectedBatch) { eb.ExpectExec("UPDATE").WithArgs(1) },
			queue:   func(b *pgx.Batch) { b.Queue("UPDATE", 2) },
			want:    "does not match actual",
		},
		"rewritten sql": {
			arrange: func(eb *ExpectedBatch) {
				eb.ExpectExec("UPDATE").WithArgs(pgx.NamedArgs{"id": 1}).WithRewrittenSQL("DELETE")
			},
			queue: func(b *pgx.Batch) { b.Queue("UPDATE t SET a = @id", pgx.NamedArgs{"id": 1}) },
			want:  "could not match actual sql",
		},
	} {
		t.Run(name, func(t *testing.T) {
			mock, err := NewConn()
			require.NoError(t, err)
			tc.arrange(mock.ExpectBatch())

			b := &pgx.Batch{}
			tc.queue(b)
			br := mock.SendBatch(ctx, b)
			_, err = br.Exec()
			assert.ErrorContains(t, err, tc.want)
			assert.ErrorContains(t, br.Close(), tc.want)
		})
	}
}

func TestBatchResultsPastTheLastQuery(t *testing.T) {
	mock, err := NewConn()
	require.NoError(t, err)
	mock.ExpectBatch().ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))

	b := &pgx.Batch{}
	b.Queue("UPDATE")
	br := mock.SendBatch(ctx, b)
	_, err = br.Exec()
	require.NoError(t, err)
	_, err = br.Exec()
	assert.ErrorContains(t, err, "no more queries in batch")
	assert.NoError(t, br.Close())
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

func TestBatchExpectedSeveralTimes(t *testing.T) {
	mock, _ := NewConn()
	// in order, the second SendBatch would still be awaited ahead of the Exec
	mock.MatchExpectationsInOrder(false)
	eb := mock.ExpectBatch().Times(2)
	eb.ExpectExec("UPDATE").Times(2).WillReturnResult(NewResult("UPDATE", 1))

	for range 2 {
		b := &pgx.Batch{}
		b.Queue("UPDATE")
		assert.NoError(t, mock.SendBatch(ctx, b).Close())
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchResultsAfterClose(t *testing.T) {
	mock, _ := NewConn()
	eb := mock.ExpectBatch()
	eb.ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))

	b := &pgx.Batch{}
	b.Queue("UPDATE")
	br := mock.SendBatch(ctx, b)
	require.NoError(t, br.Close())
	assert.NoError(t, br.Close(), "a second Close is a no-op")

	_, err := br.Exec()
	assert.ErrorIs(t, err, errBatchClosed)
}

func TestBatchResultsTx(t *testing.T) {
	mock, _ := NewConn()
	mock.ExpectBatch().ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))
	b := &pgx.Batch{}
	b.Queue("UPDATE")
	br := mock.SendBatch(ctx, b)
	assert.Nil(t, br.Tx(), "a batch without BeginTx has no transaction")
	require.NoError(t, br.Close())

	eb := mock.ExpectBatch()
	eb.ExpectExec("begin isolation level serializable").WillReturnResult(NewResult("BEGIN", 0))
	eb.ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))
	mock.ExpectCommit()
	b = &pgx.Batch{}
	b.BeginTx(pgx.TxOptions{IsoLevel: pgx.Serializable})
	b.Queue("UPDATE")
	br = mock.SendBatch(ctx, b)
	tx := br.Tx()
	require.NotNil(t, tx, "a batch with BeginTx has its transaction before Close")
	require.NoError(t, br.Close())
	assert.NoError(t, tx.Commit(ctx))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchResultsTxOfUnsentBatch(t *testing.T) {
	mock, _ := NewConn() // no batch is expected, so SendBatch fails before sending b
	b := &pgx.Batch{}
	b.BeginTx(pgx.TxOptions{})
	tx := mock.SendBatch(ctx, b).Tx()
	require.NotNil(t, tx)
	assert.ErrorIs(t, tx.Rollback(ctx), pgx.ErrTxClosed, "pgx closes the transaction of a batch it could not send")
}

func TestBatchResultsTxOfNilBatch(t *testing.T) {
	mock, _ := NewConn(ErrorOnClosedConnOption())
	mock.ExpectClose()
	require.NoError(t, mock.Close(ctx))
	br := mock.SendBatch(ctx, nil)
	assert.Nil(t, br.Tx())
	assert.ErrorIs(t, br.Close(), pgconn.ErrConnClosed)
}
