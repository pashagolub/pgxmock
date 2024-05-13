package pgxmock

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
)

type batchResults struct {
	mock          *pgxmock
	batch         *pgx.Batch
	expectedBatch *ExpectedBatch
	qqIdx         int
	err           error
}

func (br *batchResults) nextQueryAndArgs() (query string, args []any, err error) {
	if br.err != nil {
		return "", nil, br.err
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
	return br.mock.Exec(context.Background(), query, arguments...)
}

func (br *batchResults) Query() (pgx.Rows, error) {
	query, arguments, err := br.nextQueryAndArgs()
	if err != nil {
		return nil, err
	}
	return br.mock.Query(context.Background(), query, arguments...)
}

func (br *batchResults) QueryRow() pgx.Row {
	query, arguments, err := br.nextQueryAndArgs()
	if err != nil {
		return errRow{err: err}
	}
	return br.mock.QueryRow(context.Background(), query, arguments...)
}

func (br *batchResults) Close() error {
	if br.err != nil {
		return br.err
	}
	// Read and run fn for all remaining items
	for br.err == nil && br.expectedBatch != nil && !br.expectedBatch.closed && br.qqIdx < len(br.batch.QueuedQueries) {
		if qq := br.batch.QueuedQueries[br.qqIdx]; qq != nil {
			br.err = errors.Join(br.err, br.callQuedQueryFn(qq))
		}
	}
	br.expectedBatch.closed = true
	return br.err
}

func (br *batchResults) callQuedQueryFn(qq *pgx.QueuedQuery) error {
	if qq.Fn != nil {
		return qq.Fn(br)
	}
	_, err := br.Exec()
	return err
}
