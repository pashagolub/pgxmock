package pgxmock

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// BatchElement helps to create batches for testing
type BatchElement struct {
	sql          string
	rewrittenSQL string
	args         []interface{}
}

// NewBatchElement creates new query that will be queued in batch
// function accepts sql string and optionally arguments
func NewBatchElement(sql string, args ...interface{}) *BatchElement {
	return &BatchElement{
		sql:  sql,
		args: args,
	}
}

func (be *BatchElement) WithRewrittenSQL(sql string) *BatchElement {
	be.rewrittenSQL = sql
	return be
}

// Batch is a batch mock that helps to create batches for testing
type Batch struct {
	elements []*BatchElement
}

func (b *Batch) AddBatchElements(bes ...*BatchElement) *Batch {
	for _, be := range bes {
		b.elements = append(b.elements, be)
	}
	return b
}

// NewBatch creates a structure that helps to combine multiple queries that will be used in batches
// this function should be used in .ExpectSendBatch()
func NewBatch() *Batch {
	return &Batch{}
}

// BatchResults is an interface for mocking .SendBatch() response
type BatchResults interface {
	// Exec reads the results from the next query in the batch as if the query has been sent with Conn.Exec.
	Exec() (pgconn.CommandTag, error)

	// Query reads the results from the next query in the batch as if the query has been sent with Conn.Query. Prefer
	Query() (pgx.Rows, error)

	// QueryRow reads the results from the next query in the batch as if the query has been sent with Conn.QueryRow.
	QueryRow() pgx.Row

	// Close closes the batch operation.
	Close() error
}

type batchResults struct {
	err    error
	b      []*BatchElement
	closed bool
}

func NewBatchResults() BatchResults {
	return &batchResults{}
}

func (b *batchResults) Query() (pgx.Rows, error) {
	return nil, nil
}

func (b *batchResults) Exec() (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (b *batchResults) QueryRow() pgx.Row {
	return nil
}

func (b *batchResults) Close() error {
	return nil
}

func (b *batchResults) WithError(err error) {
	b.err = err
}
