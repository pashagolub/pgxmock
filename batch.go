package pgxmock

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
)

// errBatchClosed is what pgx reports for reading results of a closed batch.
var errBatchClosed = errors.New("batch already closed")

type batchResults struct {
	ctx           context.Context // of SendBatch, used for every result like in pgx
	mock          *pgxmock
	batch         *pgx.Batch
	expectedBatch *ExpectedBatch
	qqIdx         int
	err           error
	closed        bool // per results, so a reused expectation runs each batch in full
	tx            pgx.Tx
	beginFailed   bool // the begin query of pgx.Batch.BeginTx failed, so Close ends tx
}

func (br *batchResults) nextQueryAndArgs() (query string, args []any, err error) {
	if br.err != nil {
		return "", nil, br.err
	}
	if br.closed {
		return "", nil, errBatchClosed
	}
	if br.batch == nil {
		return "", nil, errors.New("no batch expectations set")
	}
	if br.qqIdx >= len(br.batch.QueuedQueries) {
		return "", nil, errors.New("no more queries in batch")
	}
	bi := br.batch.QueuedQueries[br.qqIdx]
	query = bi.SQL
	args = bi.Arguments
	br.qqIdx++
	return
}

func (br *batchResults) Exec() (pgconn.CommandTag, error) {
	query, arguments, err := br.nextQueryAndArgs()
	if err != nil {
		return pgconn.NewCommandTag(""), err
	}
	ct, err := br.mock.Exec(br.ctx, query, arguments...)
	return ct, br.checkBegin(err)
}

func (br *batchResults) Query() (pgx.Rows, error) {
	query, arguments, err := br.nextQueryAndArgs()
	if err != nil {
		return &errRows{err: err}, err // pgx never returns nil rows, so a deferred Close is safe
	}
	rows, err := br.mock.Query(br.ctx, query, arguments...)
	return rows, br.checkBegin(err)
}

func (br *batchResults) QueryRow() pgx.Row {
	query, arguments, err := br.nextQueryAndArgs()
	if err != nil {
		return errRow{err: err}
	}
	return br.mock.QueryRow(br.ctx, query, arguments...)
}

func (br *batchResults) Close() error {
	if br.err != nil {
		br.closed = true
		return br.err
	}
	if br.closed {
		return nil
	}
	// Read and run fn for all remaining items
	for br.err == nil && br.batch != nil && br.qqIdx < len(br.batch.QueuedQueries) {
		idx := br.qqIdx
		if qq := br.batch.QueuedQueries[idx]; qq != nil {
			br.err = errors.Join(br.err, br.callQuedQueryFn(qq))
		}
		if br.qqIdx == idx {
			// a nil query, or a callback that did not read its result, must
			// not keep the loop spinning on the same query forever
			br.qqIdx++
		}
	}
	br.closed = true
	if br.beginFailed {
		br.mock.txDone.Store(true)
	}
	return br.err
}

// checkBegin records an error of the begin query of pgx.Batch.BeginTx and returns err.
func (br *batchResults) checkBegin(err error) error {
	if err != nil && br.qqIdx == 1 && br.tx == br.mock {
		br.beginFailed = true
	}
	return err
}

func (br *batchResults) Tx() pgx.Tx {
	return br.tx
}

// batchTx returns what pgx gives for the transaction of b: none without pgx.Batch.BeginTx,
// a closed one when err stops b before it is sent, otherwise the mock as from BeginTx.
func (c *pgxmock) batchTx(b *pgx.Batch, err error) pgx.Tx {
	if b == nil {
		return nil
	}
	tx := pgx.FailedBatchResults(b, err).Tx()
	if tx == nil || err != nil {
		return tx
	}
	c.txDone.Store(false)
	return c
}

func (br *batchResults) callQuedQueryFn(qq *pgx.QueuedQuery) error {
	switch {
	case qq.Fn != nil:
		return qq.Fn(br)
	case br.expectsRows(br.qqIdx):
		// read and discard the result, the way it was expected
		rows, err := br.Query()
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		rows.Close()
		return rows.Err()
	default:
		_, err := br.Exec()
		return err
	}
}

// expectsRows reports whether the queued query at idx was expected with
// ExpectedBatch.ExpectQuery rather than ExpectedBatch.ExpectExec.
func (br *batchResults) expectsRows(idx int) bool {
	return br.expectedBatch != nil && idx < len(br.expectedBatch.returnsRows) && br.expectedBatch.returnsRows[idx]
}
